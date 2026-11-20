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
static const uint64_t kMirrorJoiningID = kMirrorFriendID + 100;
static const uint32_t kMirrorAppID = 480;
static const char kMirrorAchievement[] = "MIRROR_ACH";
static const int64_t kMirrorUnlockTime = 1700000000LL;
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
    std::atomic<bool> got_achievement_unlock{false};
    std::atomic<bool> got_stats_set{false};
    std::atomic<bool> got_stats_get{false};
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
                (*friend_msg->mutable_rich_presence())["persona_state"] = "0";
                (*friend_msg->mutable_rich_presence())["connect_str"] = "127.0.0.1:27015";
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

            if (mirror) {
                sapie::Frame unlock_push;
                Common_Message *unlock_steam = unlock_push.mutable_steam();
                unlock_steam->set_source_id(kMirrorFriendID);
                AchievementUnlock *unlock = unlock_steam->mutable_achievement_unlock();
                unlock->set_steam_id(kMirrorFriendID);
                unlock->set_appid(kMirrorAppID);
                unlock->set_unlock_time(kMirrorUnlockTime);
                AchievementDef *def = unlock->mutable_def();
                def->set_id(kMirrorAchievement);
                def->set_display_name("Mirror Achievement");
                if (!write_frame(fd, unlock_push)) {
                    close(fd);
                    continue;
                }
            }

            bool lobby_join_pushed = false;
            while (!stop) {
                if (!wait_readable(fd, 50)) continue;
                sapie::Frame inbound;
                if (!read_frame(fd, inbound)) break;
                if (inbound.has_ping()) {
                    sapie::Frame pong;
                    pong.set_pong(true);
                    if (!write_frame(fd, pong)) break;
                } else if (inbound.has_achievement_unlock()) {
                    got_achievement_unlock = true;
                } else if (inbound.has_stats_set()) {
                    got_stats_set = true;
                } else if (inbound.has_stats_get()) {
                    got_stats_get = true;
                    sapie::Frame response;
                    sapie::StatsResponse *stats = response.mutable_stats_response();
                    uint64_t target_id = inbound.stats_get().steam_id() ? inbound.stats_get().steam_id() : kFixedSteamID;
                    stats->set_steam_id(target_id);
                    sapie::AchievementState *achievement = stats->add_achievements();
                    achievement->set_name(kMirrorAchievement);
                    achievement->set_achieved(true);
                    achievement->set_unlock_time((uint32_t)kMirrorUnlockTime);
                    achievement->set_unlock_time_unix(kMirrorUnlockTime);
                    if (!write_frame(fd, response)) break;

                    if (mirror && !lobby_join_pushed) {
                        lobby_join_pushed = true;
                        sapie::Frame join_push;
                        Common_Message *join_steam = join_push.mutable_steam();
                        join_steam->set_source_id(kMirrorFriendID);
                        Lobby *joined_lobby = join_steam->mutable_lobby();
                        joined_lobby->set_room_id(mirror_lobby_id());
                        joined_lobby->set_owner(kMirrorFriendID);
                        joined_lobby->set_appid(kMirrorAppID);
                        joined_lobby->set_member_limit(4);
                        joined_lobby->set_type(k_ELobbyTypePublic);
                        joined_lobby->set_joinable(true);
                        (*joined_lobby->mutable_values())["name"] = "Mirror Lobby";
                        Lobby_Member *captain = joined_lobby->add_members();
                        captain->set_id(kMirrorFriendID);
                        (*captain->mutable_values())["role"] = "captain";
                        Lobby_Member *local = joined_lobby->add_members();
                        local->set_id(kFixedSteamID);
                        if (!write_frame(fd, join_push)) break;

                        sapie::Frame owned_push;
                        Common_Message *owned_steam = owned_push.mutable_steam();
                        owned_steam->set_source_id(kFixedSteamID);
                        Lobby *owned_lobby = owned_steam->mutable_lobby();
                        owned_lobby->set_room_id(mirror_lobby_id());
                        owned_lobby->set_owner(kFixedSteamID);
                        owned_lobby->set_appid(kMirrorAppID);
                        owned_lobby->set_member_limit(4);
                        owned_lobby->set_type(k_ELobbyTypePublic);
                        owned_lobby->set_joinable(true);
                        (*owned_lobby->mutable_values())["name"] = "Mirror Lobby";
                        Lobby_Member *self = owned_lobby->add_members();
                        self->set_id(kFixedSteamID);
                        if (!write_frame(fd, owned_push)) break;

                        sapie::Frame member_join_push;
                        Common_Message *member_steam = member_join_push.mutable_steam();
                        member_steam->set_source_id(kMirrorJoiningID);
                        Lobby_Messages *member_join = member_steam->mutable_lobby_messages();
                        member_join->set_type(Lobby_Messages::JOIN);
                        member_join->set_id(mirror_lobby_id());
                        if (!write_frame(fd, member_join_push)) break;
                    }
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

    if (mirror) {
        setenv("SteamAppId", "480", 1);
        std::ofstream achievements(std::string(temp_dir) + "/steam_settings/achievements.json");
        achievements << "[{\"name\":\"" << kMirrorAchievement
                     << "\",\"hidden\":0,\"displayName\":{\"english\":\"Mirror Achievement\"},"
                        "\"description\":{\"english\":\"Mirror unlock smoke\"},"
                        "\"progress\":{\"value\":{\"operation\":\"from_to\",\"operand1\":\"MIRROR_STAT\"},"
                        "\"min_val\":\"0\",\"max_val\":\"1\"}}]";
        achievements.close();
    }

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
    if (mirror) SteamAPI_ManualDispatch_Init();

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
            if (friends->GetFriendPersonaState(buddy) != k_EPersonaStateOffline) {
                printf("FAIL: explicit offline persona_state was not mirrored\n");
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
            ISteamUserStats *stats = SteamUserStats();
            if (!stats) {
                printf("FAIL: mirror run has no user stats interface\n");
                return 1;
            }

            bool achieved = false;
            uint32 unlock_time = 0;
            if (!stats->GetUserAchievementAndUnlockTime(buddy, kMirrorAchievement, &achieved, &unlock_time)
                || !achieved || unlock_time != (uint32_t)kMirrorUnlockTime) {
                printf("FAIL: peer achievement unlock was not mirrored (achieved=%d time=%u)\n",
                       (int)achieved, (unsigned)unlock_time);
                return 1;
            }

            matchmaking->JoinLobby(lobby);
            stats->RequestUserStats(buddy);
            if (!stats->SetAchievement(kMirrorAchievement) || !stats->StoreStats()) {
                printf("FAIL: achievement unlock or StoreStats was rejected\n");
                return 1;
            }
            for (int i = 0; i < 20; ++i) {
                SteamAPI_RunCallbacks();
                std::this_thread::sleep_for(std::chrono::milliseconds(50));
            }

            achieved = false;
            unlock_time = 0;
            if (!stats->GetAchievementAndUnlockTime(kMirrorAchievement, &achieved, &unlock_time)
                || !achieved || unlock_time != (uint32_t)kMirrorUnlockTime) {
                printf("FAIL: self achievement time was not canonical (achieved=%d time=%u)\n",
                       (int)achieved, (unsigned)unlock_time);
                return 1;
            }
            if (!stats->GetUserAchievementAndUnlockTime(CSteamID(steam_id), kMirrorAchievement, &achieved, &unlock_time)
                || !achieved || unlock_time != (uint32_t)kMirrorUnlockTime) {
                printf("FAIL: self peer-stat getter disagrees with local getter\n");
                return 1;
            }
            if (!server.got_achievement_unlock || !server.got_stats_get || server.got_stats_set) {
                printf("FAIL: unexpected stats frames (unlock=%d set=%d get=%d)\n",
                       (int)server.got_achievement_unlock, (int)server.got_stats_set, (int)server.got_stats_get);
                return 1;
            }

            bool got_lobby_enter_cb = false;
            bool got_member_join_cb = false;
            bool got_achievement_cb = false;
            bool got_stats_stored_cb = false;
            bool got_stats_received_cb = false;
            HSteamPipe pipe = SteamAPI_GetHSteamPipe();
            for (;;) {
                CallbackMsg_t msg = {};
                if (!SteamAPI_ManualDispatch_GetNextCallback(pipe, &msg)) break;
                if (msg.m_iCallback == LobbyEnter_t::k_iCallback
                    && msg.m_pubParam && msg.m_cubParam >= (int)sizeof(LobbyEnter_t)) {
                    auto *enter = reinterpret_cast<LobbyEnter_t *>(msg.m_pubParam);
                    if (enter->m_ulSteamIDLobby == mirror_lobby_id()
                        && enter->m_EChatRoomEnterResponse == k_EChatRoomEnterResponseSuccess) {
                        got_lobby_enter_cb = true;
                    }
                } else if (msg.m_iCallback == LobbyChatUpdate_t::k_iCallback
                    && msg.m_pubParam && msg.m_cubParam >= (int)sizeof(LobbyChatUpdate_t)) {
                    auto *update = reinterpret_cast<LobbyChatUpdate_t *>(msg.m_pubParam);
                    if (update->m_ulSteamIDLobby == mirror_lobby_id()
                        && update->m_ulSteamIDUserChanged == kMirrorJoiningID
                        && (update->m_rgfChatMemberStateChange & k_EChatMemberStateChangeEntered)) {
                        got_member_join_cb = true;
                    }
                } else if (msg.m_iCallback == UserAchievementStored_t::k_iCallback
                    && msg.m_pubParam && msg.m_cubParam >= (int)sizeof(UserAchievementStored_t)) {
                    auto *stored = reinterpret_cast<UserAchievementStored_t *>(msg.m_pubParam);
                    if (std::string(stored->m_rgchAchievementName) == kMirrorAchievement) got_achievement_cb = true;
                } else if (msg.m_iCallback == UserStatsStored_t::k_iCallback) {
                    got_stats_stored_cb = true;
                } else if (msg.m_iCallback == UserStatsReceived_t::k_iCallback) {
                    got_stats_received_cb = true;
                }
                SteamAPI_ManualDispatch_FreeLastCallback(pipe);
            }
            if (!got_lobby_enter_cb || !got_member_join_cb || !got_achievement_cb
                || !got_stats_stored_cb || !got_stats_received_cb) {
                printf("FAIL: missing Steam callbacks (lobby_enter=%d member_join=%d achievement=%d stored=%d received=%d)\n",
                       (int)got_lobby_enter_cb, (int)got_member_join_cb, (int)got_achievement_cb,
                       (int)got_stats_stored_cb, (int)got_stats_received_cb);
                return 1;
            }

            bool has_map_key = false;
            int key_count = friends->GetFriendRichPresenceKeyCount(buddy);
            for (int i = 0; i < key_count; ++i) {
                const char *key = friends->GetFriendRichPresenceKeyByIndex(buddy, i);
                if (!key) continue;
                std::string name(key);
                if (name == "persona_state" || name == "connect_str") {
                    printf("FAIL: reserved rich presence key leaked: %s\n", key);
                    return 1;
                }
                if (name == "map") has_map_key = true;
            }
            if (!has_map_key) {
                printf("FAIL: pushed rich presence key 'map' missing\n");
                return 1;
            }

            printf("PASS: mirror run loaded friends/lobby and synced achievement at %lld (callbacks ok)\n",
                   (long long)kMirrorUnlockTime);
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
