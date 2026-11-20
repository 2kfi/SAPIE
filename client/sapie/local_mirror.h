/* SAPIE client-side mirror of server-owned friend and lobby state.
   Synchronous Steam getters read this mirror; they never talk to the server. */

#ifndef SAPIE_LOCAL_MIRROR_H
#define SAPIE_LOCAL_MIRROR_H

#include <algorithm>
#include <cstdlib>
#include <map>
#include <mutex>
#include <string>
#include <tuple>
#include <utility>
#include <vector>

#include "net.pb.h"
#include "sapie.pb.h"

namespace sapie {

inline const char kPersonaStateKey[] = "persona_state";
inline const char kConnectStrKey[] = "connect_str";

class LocalMirror;
LocalMirror &local_mirror();

class LocalMirror {
    mutable std::recursive_mutex mutex;
    std::vector<Friend> friend_msgs;
    std::vector<Lobby> lobby_msgs;
    std::map<uint64, int> persona_states;
    std::map<uint64, std::string> connect_strs;
    std::map<std::tuple<uint64, uint32, std::string>, int64> achievement_unlocks;
    bool stats_store_pending = false;

    void extract_meta(Friend &f, uint64 id)
    {
        auto state = f.rich_presence().find(kPersonaStateKey);
        if (state != f.rich_presence().end()) {
            persona_states[id] = std::atoi(state->second.c_str());
            f.mutable_rich_presence()->erase(state->first);
        }
        auto connect = f.rich_presence().find(kConnectStrKey);
        if (connect != f.rich_presence().end()) {
            connect_strs[id] = connect->second;
            f.mutable_rich_presence()->erase(connect->first);
        }
    }

public:
    Friend without_meta(const Friend &incoming) const
    {
        Friend copy = incoming;
        copy.mutable_rich_presence()->erase(kPersonaStateKey);
        copy.mutable_rich_presence()->erase(kConnectStrKey);
        return copy;
    }

    std::vector<Friend> &friends() { return friend_msgs; }
    std::vector<Lobby> &lobbies() { return lobby_msgs; }

    void apply_login(const sapie::LoginResponse &response)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        friend_msgs.clear();
        lobby_msgs.clear();
        for (const auto &snapshot : response.friends()) {
            Friend f;
            f.set_id(snapshot.steam_id());
            f.set_name(snapshot.persona_name());
            f.set_appid(snapshot.appid());
            f.set_lobby_id(snapshot.lobby_id());
            *f.mutable_rich_presence() = snapshot.rich_presence();
            upsert_friend(f);
        }
    }

    void upsert_friend(const Friend &f)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        Friend stored = f;
        extract_meta(stored, f.id());
        for (auto &existing : friend_msgs) {
            if (existing.id() == f.id()) {
                existing = stored;
                return;
            }
        }
        friend_msgs.push_back(stored);
    }

    void erase_friend(uint64 id)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        for (auto it = friend_msgs.begin(); it != friend_msgs.end(); ++it) {
            if (it->id() == id) {
                friend_msgs.erase(it);
                return;
            }
        }
    }

    void set_friend_lobby(uint64 id, uint64 lobby_id)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        for (auto &f : friend_msgs) {
            if (f.id() == id) {
                f.set_lobby_id(lobby_id);
                return;
            }
        }
    }

    void set_connect_str(uint64 id, const std::string &value)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        connect_strs[id] = value;
    }

    int persona_state(uint64 id) const
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        auto it = persona_states.find(id);
        return it == persona_states.end() ? -1 : it->second;
    }

    std::string connect_str(uint64 id) const
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        auto it = connect_strs.find(id);
        return it == connect_strs.end() ? std::string() : it->second;
    }

    void upsert_lobby(const Lobby &l)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        for (auto &existing : lobby_msgs) {
            if (existing.room_id() == l.room_id()) {
                existing = l;
                return;
            }
        }
        lobby_msgs.push_back(l);
    }

    void erase_lobby(uint64 room_id)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        for (auto it = lobby_msgs.begin(); it != lobby_msgs.end(); ++it) {
            if (it->room_id() == room_id) {
                lobby_msgs.erase(it);
                return;
            }
        }
    }

    void apply_steam_message(Common_Message *msg)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        if (msg->has_friend_()) upsert_friend(msg->friend_());

        if (msg->has_friend_messages()) {
            const auto &fm = msg->friend_messages();
            if (fm.type() == Friend_Messages::GAME_INVITE && msg->source_id())
                set_connect_str(msg->source_id(), fm.connect_str());
            if (fm.type() == Friend_Messages::LOBBY_INVITE && msg->source_id())
                set_friend_lobby(msg->source_id(), fm.lobby_id());
        }

        if (msg->has_lobby()) {
            if (msg->lobby().deleted()) erase_lobby(msg->lobby().room_id());
            else upsert_lobby(msg->lobby());
        }

        if (msg->has_lobby_messages()) {
            const auto &lm = msg->lobby_messages();
            for (auto &lobby : lobby_msgs) {
                if (lobby.room_id() != lm.id()) continue;
                auto member = std::find_if(lobby.mutable_members()->begin(), lobby.mutable_members()->end(),
                    [&msg](const Lobby_Member &m) { return m.id() == msg->source_id(); });
                if (lm.type() == Lobby_Messages::JOIN && member == lobby.mutable_members()->end()) {
                    auto added = lobby.add_members();
                    added->set_id(msg->source_id());
                } else if (lm.type() == Lobby_Messages::LEAVE && member != lobby.mutable_members()->end()) {
                    lobby.mutable_members()->erase(member);
                } else if (lm.type() == Lobby_Messages::MEMBER_DATA && member != lobby.mutable_members()->end()) {
                    for (const auto &entry : lm.map()) (*member->mutable_values())[entry.first] = entry.second;
                } else if (lm.type() == Lobby_Messages::CHANGE_OWNER) {
                    lobby.set_owner(lm.idata());
                }
                break;
            }
        }
    }

    bool set_achievement_unlock(uint64 steam_id, uint32 appid, const std::string &name, int64 unlock_time)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        if (!steam_id || name.empty() || unlock_time <= 0) return false;

        auto key = std::make_tuple(steam_id, appid, name);
        auto it = achievement_unlocks.find(key);
        if (it != achievement_unlocks.end() && it->second >= unlock_time) return false;
        achievement_unlocks[key] = unlock_time;
        return true;
    }

    bool apply_achievement_unlock(const AchievementUnlock &unlock, uint64 fallback_id = 0)
    {
        if (!unlock.has_def() || unlock.def().id().empty()) return false;
        uint64 steam_id = unlock.steam_id() ? unlock.steam_id() : fallback_id;
        return set_achievement_unlock(steam_id, unlock.appid(), unlock.def().id(), unlock.unlock_time());
    }

    std::vector<std::string> apply_stats_response(const sapie::StatsResponse &response, uint32 appid)
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        std::vector<std::string> added;
        for (const auto &achievement : response.achievements()) {
            if (achievement.achieved()
                && set_achievement_unlock(response.steam_id(), appid, achievement.name(), achievement.unlock_time_unix())) {
                added.push_back(achievement.name());
            }
        }
        return added;
    }

    bool achievement_unlocked(uint64 steam_id, uint32 appid, const std::string &name) const
    {
        return achievement_unlock_time(steam_id, appid, name) > 0;
    }

    int64 achievement_unlock_time(uint64 steam_id, uint32 appid, const std::string &name) const
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        auto it = achievement_unlocks.find(std::make_tuple(steam_id, appid, name));
        return it == achievement_unlocks.end() ? 0 : it->second;
    }

    void note_stats_store()
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        stats_store_pending = true;
    }

    bool consume_stats_store()
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        bool pending = stats_store_pending;
        stats_store_pending = false;
        return pending;
    }

    std::vector<std::pair<uint64, uint64>> friend_lobbies() const
    {
        std::lock_guard<std::recursive_mutex> lock(mutex);
        std::vector<std::pair<uint64, uint64>> result;
        for (const auto &f : friend_msgs) {
            if (f.lobby_id()) result.emplace_back(f.id(), f.lobby_id());
        }
        return result;
    }
};

inline LocalMirror &local_mirror()
{
    static LocalMirror mirror;
    return mirror;
}

} // namespace sapie

#endif
