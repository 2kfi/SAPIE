/* SAPIE M1 smoke test: stub SAPIE server + SteamAPI_Init identity check. */

#include <steam_api.h>
#include "sapie.pb.h"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/select.h>
#include <sys/socket.h>
#include <unistd.h>

#include <atomic>
#include <chrono>
#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <string>
#include <thread>

static const uint64_t kFixedSteamID = 76561197960265728ULL + 4242;
static const uint64_t kMirrorFriendID = kFixedSteamID + 1;
static const uint32_t kMirrorAppID = 480;
static const int kMaxFrameSize = 1024 * 1024;

static uint64_t mirror_lobby_id()
{
    return CSteamID(4243, k_EChatInstanceFlagLobby | k_EChatInstanceFlagMMSLobby,
                    k_EUniversePublic, k_EAccountTypeChat).ConvertToUint64();
}

static bool read_exact(int fd, char *data, size_t length)
{
    size_t received = 0;
    while (received < length) {
        ssize_t result = recv(fd, data + received, length - received, 0);
        if (result > 0) {
            received += (size_t)result;
            continue;
        }
        if (result < 0 && (errno == EAGAIN || errno == EINTR)) continue;
        return false;
    }
    return true;
}

static bool write_all(int fd, const char *data, size_t length)
{
    size_t sent = 0;
    while (sent < length) {
        ssize_t result = send(fd, data + sent, length - sent, MSG_NOSIGNAL);
        if (result > 0) {
            sent += (size_t)result;
            continue;
        }
        if (result < 0 && (errno == EAGAIN || errno == EINTR)) continue;
        return false;
    }
    return true;
}

static bool read_frame(int fd, sapie::Frame &frame)
{
    uint32_t length = 0;
    if (!read_exact(fd, (char *)&length, sizeof(length))) return false;
    length = ntohl(length);
    if (length == 0 || length > kMaxFrameSize) return false;

    std::string bytes(length, '\0');
    if (!read_exact(fd, &bytes[0], length)) return false;
    return frame.ParseFromArray(bytes.data(), (int)length);
}

static bool write_frame(int fd, const sapie::Frame &frame)
{
    std::string bytes;
    if (!frame.SerializeToString(&bytes)) return false;
    uint32_t length = htonl((uint32_t)bytes.size());
    return write_all(fd, (const char *)&length, sizeof(length)) && write_all(fd, bytes.data(), bytes.size());
}

static bool wait_readable(int fd, int timeout_ms)
{
    fd_set fds;
    FD_ZERO(&fds);
    FD_SET(fd, &fds);
    struct timeval timeout;
    timeout.tv_sec = timeout_ms / 1000;
    timeout.tv_usec = (timeout_ms % 1000) * 1000;
    return select(fd + 1, &fds, NULL, NULL, &timeout) > 0;
}

struct StubServer {
    int listen_fd = -1;
    uint16_t port = 0;
    std::atomic<bool> got_login{false};
    std::atomic<bool> correct_credentials{false};
    std::atomic<int> connections{0};
    std::atomic<bool> stop{false};
    bool reject;
    bool mirror;
    std::thread worker;

    StubServer(bool reject_, bool mirror_) : reject(reject_), mirror(mirror_)
    {
        listen_fd = socket(AF_INET, SOCK_STREAM, 0);
        if (listen_fd < 0) {
            perror("socket");
            exit(2);
        }

        struct sockaddr_in address = {};
        address.sin_family = AF_INET;
        address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        address.sin_port = 0;
        if (bind(listen_fd, (struct sockaddr *)&address, sizeof(address)) != 0 || listen(listen_fd, 1) != 0) {
            perror("bind/listen");
            exit(2);
        }

        socklen_t address_length = sizeof(address);
        getsockname(listen_fd, (struct sockaddr *)&address, &address_length);
        port = ntohs(address.sin_port);
        worker = std::thread(&StubServer::run, this);
    }

    ~StubServer()
    {
        stop = true;
        if (listen_fd >= 0) close(listen_fd);
        if (worker.joinable()) worker.join();
    }

    void run()
    {
        while (!stop && wait_readable(listen_fd, 100)) {
            int fd = accept(listen_fd, NULL, NULL);
            if (fd < 0) return;
            ++connections;

            sapie::Frame frame;
            if (!read_frame(fd, frame) || !frame.has_login()) {
                close(fd);
                continue;
            }

            got_login = true;
            correct_credentials = frame.login().username() == "smoke" && frame.login().password() == "secret";

            sapie::Frame response;
            sapie::LoginResponse *login_response = response.mutable_login_response();
            login_response->set_status(reject ? sapie::LoginResponse_Status_BAD_CREDENTIALS
                                              : sapie::LoginResponse_Status_OK);
            login_response->set_steam_id(reject ? 0 : kFixedSteamID);
            login_response->set_persona_name(reject ? "" : "SmokeTester");
            login_response->set_session_token(reject ? "" : "token");
            if (mirror) {
                sapie::FriendSnapshot *buddy = login_response->add_friends();
                buddy->set_steam_id(kMirrorFriendID);
                buddy->set_persona_name("MirrorBuddy");
                buddy->set_appid(kMirrorAppID);
                buddy->set_lobby_id(mirror_lobby_id());
                (*buddy->mutable_rich_presence())["persona_state"] = "2";
                (*buddy->mutable_rich_presence())["map"] = "de_dust";
            }
            if (!write_frame(fd, response)) {
                close(fd);
                continue;
            }

            if (mirror) {
                sapie::Frame friend_push;
                Common_Message *friend_steam = friend_push.mutable_steam();
                friend_steam->set_source_id(kMirrorFriendID);
                Friend *friend_msg = friend_steam->mutable_friend_();
                friend_msg->set_id(kMirrorFriendID);
                friend_msg->set_name("MirrorBuddy");
                friend_msg->set_appid(kMirrorAppID);
                friend_msg->set_lobby_id(mirror_lobby_id());
                (*friend_msg->mutable_rich_presence())["persona_state"] = "1";
                (*friend_msg->mutable_rich_presence())["map"] = "de_dust";
                if (!write_frame(fd, friend_push)) {
                    close(fd);
                    continue;
                }
            }

            if (mirror) {
                sapie::Frame push;
                Common_Message *steam = push.mutable_steam();
                steam->set_source_id(kMirrorFriendID);
                Lobby *lobby = steam->mutable_lobby();
                lobby->set_room_id(mirror_lobby_id());
                lobby->set_owner(kMirrorFriendID);
                lobby->set_appid(kMirrorAppID);
                lobby->set_member_limit(4);
                lobby->set_type(k_ELobbyTypePublic);
                lobby->set_joinable(true);
                (*lobby->mutable_values())["name"] = "Mirror Lobby";
                Lobby_Member *member = lobby->add_members();
                member->set_id(kMirrorFriendID);
                (*member->mutable_values())["role"] = "captain";
                if (!write_frame(fd, push)) {
                    close(fd);
                    continue;
                }
            }

            while (!stop) {
                if (!wait_readable(fd, 50)) continue;
                sapie::Frame inbound;
                if (!read_frame(fd, inbound)) break;
                if (inbound.has_ping()) {
                    sapie::Frame pong;
                    pong.set_pong(true);
                    if (!write_frame(fd, pong)) break;
                }
            }
            close(fd);
        }
    }
};

int main(int argc, char **argv)
{
    std::string mode = argc > 1 ? argv[1] : "";
    bool reject = mode == "--reject";
    bool mirror = mode == "--mirror";
    bool expect_login = mode.empty() || reject || mirror;
    bool fallback = mode == "--fallback";

    char temp_pattern[] = "/tmp/sapie-w1a-smoke-XXXXXX";
    char *temp_dir = mkdtemp(temp_pattern);
    if (!temp_dir) {
        perror("mkdtemp");
        return 2;
    }

    std::filesystem::create_directories(std::string(temp_dir) + "/steam_settings");
    std::filesystem::create_directories(std::string(temp_dir) + "/data");
    setenv("SteamAppPath", temp_dir, 1);
    setenv("XDG_DATA_HOME", (std::string(temp_dir) + "/data").c_str(), 1);

    if (mirror) setenv("SteamAppId", "480", 1);

    StubServer server(reject, mirror);
    if (expect_login) {
        std::ofstream config(std::string(temp_dir) + "/steam_settings/sapie_server.txt");
        config << "127.0.0.1:" << server.port << "\n";
        config.close();
        setenv("SAPIE_USERNAME", "smoke", 1);
        setenv("SAPIE_PASSWORD", "secret", 1);
    } else if (fallback) {
        // Config at the program root (not steam_settings) with missing credentials.
        std::ofstream config(std::string(temp_dir) + "/sapie_server.txt");
        config << "127.0.0.1:" << server.port << "  \r\n";
        config.close();
        unsetenv("SAPIE_USERNAME");
        unsetenv("SAPIE_PASSWORD");
    }

    if (!SteamAPI_Init()) {
        printf("FAIL: SteamAPI_Init returned false\n");
        return 1;
    }

    ISteamUser *user = SteamUser();
    uint64 steam_id = user ? user->GetSteamID().ConvertToUint64() : 0;

    if (expect_login) {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        if (!server.got_login || !server.correct_credentials) {
            printf("FAIL: stub server did not receive the expected LoginRequest\n");
            return 1;
        }
        if (reject) {
            if (steam_id == kFixedSteamID) {
                printf("FAIL: rejected login still used server SteamID\n");
                return 1;
            }
            printf("PASS: rejected login kept local SteamID %llu\n", (unsigned long long)steam_id);
            return 0;
        }
        if (steam_id != kFixedSteamID) {
            printf("FAIL: expected SteamID %llu, got %llu\n", (unsigned long long)kFixedSteamID, (unsigned long long)steam_id);
            return 1;
        }
        if (mirror) {
            ISteamFriends *friends = SteamFriends();
            ISteamMatchmaking *matchmaking = SteamMatchmaking();
            if (!friends || !matchmaking) {
                printf("FAIL: mirror run has no friends or matchmaking interface\n");
                return 1;
            }
            CSteamID buddy((uint64)kMirrorFriendID);
            if (friends->GetFriendPersonaState(buddy) != k_EPersonaStateBusy) {
                printf("FAIL: login-seeded persona_state was not mirrored\n");
                return 1;
            }

            for (int i = 0; i < 10; ++i) {
                SteamAPI_RunCallbacks();
                std::this_thread::sleep_for(std::chrono::milliseconds(50));
            }

            if (friends->GetFriendCount(k_EFriendFlagImmediate) != 1) {
                printf("FAIL: mirror run did not load the pushed friend\n");
                return 1;
            }
            if (friends->GetFriendByIndex(0, k_EFriendFlagImmediate).ConvertToUint64() != kMirrorFriendID) {
                printf("FAIL: unexpected friend index 0\n");
                return 1;
            }
            if (std::string(friends->GetFriendPersonaName(buddy)) != "MirrorBuddy") {
                printf("FAIL: pushed friend name was not mirrored\n");
                return 1;
            }
            if (friends->GetFriendPersonaState(buddy) != k_EPersonaStateOnline) {
                printf("FAIL: pushed persona_state was not mirrored\n");
                return 1;
            }
            FriendGameInfo_t game_info = {};
            if (!friends->GetFriendGamePlayed(buddy, &game_info)
                || game_info.m_gameID.AppID() != kMirrorAppID
                || game_info.m_steamIDLobby.ConvertToUint64() != mirror_lobby_id()) {
                printf("FAIL: pushed friend game/lobby was not mirrored\n");
                return 1;
            }
            if (std::string(friends->GetFriendRichPresence(buddy, "map")) != "de_dust") {
                printf("FAIL: pushed friend rich presence was not mirrored\n");
                return 1;
            }

            CSteamID lobby((uint64)mirror_lobby_id());
            if (matchmaking->GetLobbyOwner(lobby).ConvertToUint64() != kMirrorFriendID) {
                printf("FAIL: pushed lobby owner was not mirrored (got %llu)\n",
                       (unsigned long long)matchmaking->GetLobbyOwner(lobby).ConvertToUint64());
                return 1;
            }
            if (matchmaking->GetNumLobbyMembers(lobby) != 1
                || matchmaking->GetLobbyMemberByIndex(lobby, 0).ConvertToUint64() != kMirrorFriendID) {
                printf("FAIL: pushed lobby members were not mirrored\n");
                return 1;
            }
            if (std::string(matchmaking->GetLobbyData(lobby, "name")) != "Mirror Lobby") {
                printf("FAIL: pushed lobby data was not mirrored\n");
                return 1;
            }
            if (std::string(matchmaking->GetLobbyMemberData(lobby, buddy, "role")) != "captain") {
                printf("FAIL: pushed lobby member data was not mirrored\n");
                return 1;
            }
            printf("PASS: mirror run loaded friends and lobby from server pushes (persona_state=%d)\n",
                   (int)friends->GetFriendPersonaState(buddy));
            return 0;
        }
        printf("PASS: login used server SteamID %llu\n", (unsigned long long)steam_id);
        return 0;
    }

    std::this_thread::sleep_for(std::chrono::milliseconds(300));
    if (server.connections) {
        printf("FAIL: %s run still connected to the SAPIE server\n", mode.c_str());
        return 1;
    }
    if (!steam_id) {
        printf("FAIL: %s run has no local SteamID\n", mode.c_str());
        return 1;
    }
    printf("PASS: %s run stayed on LAN path with local SteamID %llu\n", mode.c_str(), (unsigned long long)steam_id);
    return 0;
}
