/* SAPIE client-side mirror of server-owned friend and lobby state.
   Synchronous Steam getters read this mirror; they never talk to the server. */

#ifndef SAPIE_LOCAL_MIRROR_H
#define SAPIE_LOCAL_MIRROR_H

#include <algorithm>
#include <cstdlib>
#include <map>
#include <mutex>
#include <string>
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

    static void extract_meta(Friend &f, uint64 id)
    {
        LocalMirror &mirror = local_mirror();
        auto state = f.rich_presence().find(kPersonaStateKey);
        if (state != f.rich_presence().end()) {
            mirror.persona_states[id] = std::atoi(state->second.c_str());
            f.mutable_rich_presence()->erase(state->first);
        }
        auto connect = f.rich_presence().find(kConnectStrKey);
        if (connect != f.rich_presence().end()) {
            mirror.connect_strs[id] = connect->second;
            f.mutable_rich_presence()->erase(connect->first);
        }
    }

public:
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
        return it == persona_states.end() ? 0 : it->second;
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
