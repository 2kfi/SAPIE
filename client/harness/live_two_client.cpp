/* M13 live harness: two C++ clients against one real sapie-server. */
#include <steam_api.h>

#include <unistd.h>

#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <iterator>
#include <string>
#include <thread>

namespace {

const char kAchievement[] = "M13_LIVE_ACH";
const char kInvite[] = "connect=127.0.0.1:27015";

std::string env(const char *name, const char *fallback = "") {
    const char *value = std::getenv(name);
    return value ? value : fallback;
}

bool write_file(const std::string &path, const std::string &content) {
    std::ofstream out(path);
    if (!out) return false;
    out << content;
    return out.good();
}

std::string read_file(const std::string &path) {
    std::ifstream in(path);
    if (!in) return "";
    return std::string(std::istreambuf_iterator<char>(in), std::istreambuf_iterator<char>());
}

struct Observed {
    int lobby_created = 0;
    int lobby_enter = 0;
    int lobby_member_join = 0;
    int game_invite = 0;
    int achievement_stored = 0;
    int stats_received = 0;
    uint64_t created_lobby = 0;
};

void drain_callbacks(Observed &seen) {
    HSteamPipe pipe = SteamAPI_GetHSteamPipe();
    for (;;) {
        CallbackMsg_t msg = {};
        if (!SteamAPI_ManualDispatch_GetNextCallback(pipe, &msg)) break;
        if (msg.m_iCallback == LobbyCreated_t::k_iCallback && msg.m_pubParam &&
            msg.m_cubParam >= static_cast<int>(sizeof(LobbyCreated_t))) {
            auto *created = reinterpret_cast<LobbyCreated_t *>(msg.m_pubParam);
            if (created->m_eResult == k_EResultOK) {
                ++seen.lobby_created;
                seen.created_lobby = created->m_ulSteamIDLobby;
            }
        } else if (msg.m_iCallback == LobbyEnter_t::k_iCallback && msg.m_pubParam &&
                   msg.m_cubParam >= static_cast<int>(sizeof(LobbyEnter_t))) {
            auto *enter = reinterpret_cast<LobbyEnter_t *>(msg.m_pubParam);
            if (enter->m_EChatRoomEnterResponse == k_EChatRoomEnterResponseSuccess) ++seen.lobby_enter;
        } else if (msg.m_iCallback == LobbyChatUpdate_t::k_iCallback && msg.m_pubParam &&
                   msg.m_cubParam >= static_cast<int>(sizeof(LobbyChatUpdate_t))) {
            auto *update = reinterpret_cast<LobbyChatUpdate_t *>(msg.m_pubParam);
            if (update->m_rgfChatMemberStateChange & k_EChatMemberStateChangeEntered) ++seen.lobby_member_join;
        } else if (msg.m_iCallback == GameRichPresenceJoinRequested_t::k_iCallback && msg.m_pubParam &&
                   msg.m_cubParam >= static_cast<int>(sizeof(GameRichPresenceJoinRequested_t))) {
            auto *invite = reinterpret_cast<GameRichPresenceJoinRequested_t *>(msg.m_pubParam);
            if (std::string(invite->m_rgchConnect) == kInvite) ++seen.game_invite;
        } else if (msg.m_iCallback == UserAchievementStored_t::k_iCallback && msg.m_pubParam &&
                   msg.m_cubParam >= static_cast<int>(sizeof(UserAchievementStored_t))) {
            auto *stored = reinterpret_cast<UserAchievementStored_t *>(msg.m_pubParam);
            if (std::string(stored->m_rgchAchievementName) == kAchievement) ++seen.achievement_stored;
        } else if (msg.m_iCallback == UserStatsReceived_t::k_iCallback) {
            ++seen.stats_received;
        }
        SteamAPI_ManualDispatch_FreeLastCallback(pipe);
    }
}

template <typename Predicate>
bool wait_for(int timeout_ms, Predicate predicate) {
    const auto deadline = std::chrono::steady_clock::now() + std::chrono::milliseconds(timeout_ms);
    while (std::chrono::steady_clock::now() < deadline) {
        SteamAPI_RunCallbacks();
        if (predicate()) return true;
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
    return predicate();
}

}  // namespace

int main(int argc, char **argv) {
    const std::string role = argc > 1 ? argv[1] : "";
    if (role != "host" && role != "guest") {
        fprintf(stderr, "usage: live_two_client host|guest\n");
        return 2;
    }

    char pattern[] = "/tmp/opencode/sapie-m13-live-XXXXXX";
    char *root = mkdtemp(pattern);
    if (!root) {
        perror("mkdtemp");
        return 2;
    }
    const std::string base(root);
    std::filesystem::create_directories(base + "/steam_settings");
    std::filesystem::create_directories(base + "/data");

    const std::string appid = env("SAPIE_LIVE_APPID", "480");
    const std::string server = env("SAPIE_LIVE_SERVER", "127.0.0.1:47584");
    const std::string friend_id = env("SAPIE_LIVE_FRIEND", "0");
    const std::string lobby_file = env("SAPIE_LIVE_LOBBY_FILE", (base + "/lobby_id.txt").c_str());
    if (!write_file(base + "/steam_settings/sapie_server.txt", server + "\n") ||
        !write_file(base + "/steam_appid.txt", appid + "\n") ||
        !write_file(base + "/steam_settings/achievements.json",
                    "[{\"name\":\"" + std::string(kAchievement) +
                        "\",\"hidden\":0,\"displayName\":{\"english\":\"M13 Live Achievement\"},"
                        "\"description\":{\"english\":\"M13 live two-client check\"}}]")) {
        fprintf(stderr, "FAIL %s: write config\n", role.c_str());
        return 2;
    }

    setenv("SteamAppPath", base.c_str(), 1);
    setenv("XDG_DATA_HOME", (base + "/data").c_str(), 1);
    setenv("SteamAppId", appid.c_str(), 1);
    setenv("SAPIE_USERNAME", env("SAPIE_LIVE_USERNAME").c_str(), 1);
    setenv("SAPIE_PASSWORD", env("SAPIE_LIVE_PASSWORD").c_str(), 1);

    if (!SteamAPI_Init()) {
        printf("FAIL %s: SteamAPI_Init returned false\n", role.c_str());
        return 1;
    }
    SteamAPI_ManualDispatch_Init();

    const uint64_t own_id = SteamUser()->GetSteamID().ConvertToUint64();
    const uint64_t peer = strtoull(friend_id.c_str(), nullptr, 10);
    printf("CLIENT role=%s steam_id=%llu friend=%llu server=%s appid=%s\n", role.c_str(),
           (unsigned long long)own_id, (unsigned long long)peer, server.c_str(), appid.c_str());
    fflush(stdout);

    Observed seen;
    if (!peer || !wait_for(8000, [&] {
            drain_callbacks(seen);
            return SteamFriends()->GetFriendPersonaState(CSteamID(static_cast<uint64>(peer))) == k_EPersonaStateOnline;
        })) {
        printf("FAIL %s: friend never appeared online (state=%d)\n", role.c_str(),
               (int)SteamFriends()->GetFriendPersonaState(CSteamID(static_cast<uint64>(peer))));
        return 1;
    }
    printf("PASS %s presence friend=%llu state=online\n", role.c_str(), (unsigned long long)peer);
    fflush(stdout);

    ISteamMatchmaking *matchmaking = SteamMatchmaking();
    ISteamUserStats *stats = SteamUserStats();
    if (!matchmaking || !stats) {
        printf("FAIL %s: missing matchmaking or stats interface\n", role.c_str());
        return 1;
    }

    if (role == "host") {
        if (!matchmaking->CreateLobby(k_ELobbyTypePublic, 4)) {
            printf("FAIL host: CreateLobby was rejected\n");
            return 1;
        }
        if (!wait_for(8000, [&] {
                drain_callbacks(seen);
                return seen.created_lobby != 0;
            })) {
            printf("FAIL host: LobbyCreated_t never arrived\n");
            return 1;
        }
        const uint64_t lobby = seen.created_lobby;
        printf("PASS host lobby_created lobby_id=%llu\n", (unsigned long long)lobby);
        if (!write_file(lobby_file, std::to_string(lobby) + "\n")) {
            printf("FAIL host: cannot write lobby file\n");
            return 1;
        }
        fflush(stdout);

        if (!SteamFriends()->InviteUserToGame(CSteamID(static_cast<uint64>(peer)), kInvite)) {
            printf("FAIL host: InviteUserToGame was rejected\n");
            return 1;
        }
        printf("PASS host invite_sent to=%llu connect_str=%s\n", (unsigned long long)peer, kInvite);
        if (!stats->SetAchievement(kAchievement) || !stats->StoreStats()) {
            printf("FAIL host: achievement push was rejected\n");
            return 1;
        }
        printf("PASS host achievement_pushed name=%s\n", kAchievement);
        printf("INFO host waiting for guest in lobby\n");
        fflush(stdout);
        if (!wait_for(25000, [&] {
                drain_callbacks(seen);
                return matchmaking->GetNumLobbyMembers(CSteamID(static_cast<uint64>(lobby))) == 2 && seen.achievement_stored;
            })) {
            printf("FAIL host: join or achievement callback missing (members=%d stored=%d)\n",
                   matchmaking->GetNumLobbyMembers(CSteamID(static_cast<uint64>(lobby))), seen.achievement_stored);
            return 1;
        }
        printf("PASS host joined_lobby lobby_id=%llu members=%d achievement_stored=%d\n",
               (unsigned long long)lobby, matchmaking->GetNumLobbyMembers(CSteamID(static_cast<uint64>(lobby))), seen.achievement_stored);
        fflush(stdout);
    } else {
        uint64_t lobby = 0;
        if (!wait_for(10000, [&] {
                const std::string text = read_file(lobby_file);
                if (!text.empty()) lobby = strtoull(text.c_str(), nullptr, 10);
                return lobby != 0;
            })) {
            printf("FAIL guest: no lobby ID from host\n");
            return 1;
        }
        printf("INFO guest joining lobby\n");
        fflush(stdout);
        if (!matchmaking->JoinLobby(CSteamID(static_cast<uint64>(lobby)))) {
            printf("FAIL guest: JoinLobby was rejected\n");
            return 1;
        }
        printf("PASS guest join_requested lobby_id=%llu\n", (unsigned long long)lobby);
        fflush(stdout);
        if (!wait_for(25000, [&] {
                drain_callbacks(seen);
                return seen.lobby_enter && matchmaking->GetNumLobbyMembers(CSteamID(static_cast<uint64>(lobby))) == 2 &&
                       seen.game_invite;
            })) {
            printf("FAIL guest: lobby or invite missing (enter=%d members=%d invite=%d)\n", seen.lobby_enter,
                   matchmaking->GetNumLobbyMembers(CSteamID(static_cast<uint64>(lobby))), seen.game_invite);
            return 1;
        }
        printf("PASS guest joined_lobby lobby_id=%llu members=%d invite=%d\n", (unsigned long long)lobby,
               matchmaking->GetNumLobbyMembers(CSteamID(static_cast<uint64>(lobby))), seen.game_invite);
        fflush(stdout);

        stats->RequestUserStats(CSteamID(static_cast<uint64>(peer)));
        bool achieved = false;
        uint32 unlock_time = 0;
        if (!wait_for(25000, [&] {
                drain_callbacks(seen);
                return stats->GetUserAchievementAndUnlockTime(CSteamID(static_cast<uint64>(peer)), kAchievement, &achieved, &unlock_time) &&
                       achieved && unlock_time > 0 && seen.stats_received;
            })) {
            printf("FAIL guest: peer achievement not mirrored (achieved=%d time=%u received=%d)\n", (int)achieved,
                   unlock_time, seen.stats_received);
            return 1;
        }
        printf("PASS guest achievement_from_host name=%s unlock_time=%u stats_received=%d\n", kAchievement,
               unlock_time, seen.stats_received);
        fflush(stdout);
    }

    drain_callbacks(seen);
    printf("DONE role=%s steam_id=%llu created=%d enter=%d member_join=%d invite=%d stored=%d\n", role.c_str(),
           (unsigned long long)own_id, seen.lobby_created, seen.lobby_enter, seen.lobby_member_join,
           seen.game_invite, seen.achievement_stored);
    return 0;
}
