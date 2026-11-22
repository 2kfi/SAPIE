/*
 * Calls every exported function in sapie_core.h. Built with plain cc, no Qt.
 * Run from the launcher directory after `cargo build --release`.
 */
#include "sapie_core.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <sys/stat.h>
#include <unistd.h>
#endif

static int failures = 0;
static int event_count = 0;

#define CHECK(condition)                                                       \
    do {                                                                       \
        if (!(condition)) {                                                    \
            fprintf(stderr, "FAIL line %d: %s\n", __LINE__, #condition);       \
            failures++;                                                        \
        }                                                                      \
    } while (0)

static void on_event(void *user_data, const char *step, const char *message)
{
    CHECK(user_data != NULL);
    CHECK(step != NULL && step[0] != '\0');
    CHECK(message != NULL && message[0] != '\0');
    event_count++;
}

static void write_file(const char *path, const void *data, size_t length)
{
    FILE *file = fopen(path, "wb");
    CHECK(file != NULL);
    if (file == NULL) {
        return;
    }
    CHECK(fwrite(data, 1, length, file) == length);
    fclose(file);
}

static void make_game(char *dir, size_t dir_cap, char *exe, size_t exe_cap)
{
    static const unsigned char pe[0x100] = {
        'M', 'Z', 0x80, 0x00, 0x00, 0x00,
        /* e_lfanew at 0x3c */
    };
    static const unsigned char dll[] = {
        'S','t','e','a','m','U','s','e','r','0','1','9', 0,
        'S','t','e','a','m','C','l','i','e','n','t','0','1','7', 0
    };
    unsigned char image[0x100];
    char path[512];

#ifdef _WIN32
    char temp_root[MAX_PATH];
    GetTempPathA(sizeof(temp_root), temp_root);
    snprintf(dir, dir_cap, "%ssapie-c-smoke-%ld", temp_root, (long)getpid());
    CreateDirectoryA(dir, NULL);
#else
    snprintf(dir, dir_cap, "/tmp/sapie-c-smoke-%ld", (long)getpid());
    mkdir(dir, 0700);
#endif
    memset(image, 0, sizeof(image));
    memcpy(image, pe, sizeof(pe));
    image[0x3c] = 0x80;
    image[0x80] = 'P';
    image[0x81] = 'E';
    image[0x84] = 0x64;
    image[0x85] = 0x86;
    snprintf(exe, exe_cap, "%s/game.exe", dir);
    write_file(exe, image, sizeof(image));
    snprintf(path, sizeof(path), "%s/steam_api64.dll.sapie-original.bak", dir);
    write_file(path, dll, sizeof(dll));
}

int main(void)
{
    SapieCore *core = NULL;
    SapieCore *rejected = NULL;
    SapieSessionInfo *session = NULL;
    SapieDetectResult *detected = NULL;
    SapieInstallResult *installed = NULL;
    SapieConfigureResult *configured = NULL;
    SapieGameRecord *record = NULL;
    SapieGameRecord *records = NULL;
    char *restored = NULL;
    char *unpacked = NULL;
    char *text = NULL;
    char *preference = NULL;
    char *folder = NULL;
    char *version = NULL;
    char *update = NULL;
    char *join_id = NULL;
    char **strings = NULL;
    char dir[256];
    char exe[320];
    size_t count = 99;
    uint32_t pid = 99;
    int marker = 7;

    CHECK(sapie_core_abi_version() == SAPIE_CORE_ABI_VERSION);

    CHECK(sapie_core_create(sapie_core_abi_version() + 1, NULL, NULL, &rejected)
          == SAPIE_ERR_ABI_VERSION);
    CHECK(rejected == NULL);
    CHECK(sapie_core_create(SAPIE_CORE_ABI_VERSION, on_event, &marker, NULL)
          == SAPIE_ERR_INVALID_ARGUMENT);

    CHECK(sapie_core_create_fake(SAPIE_CORE_ABI_VERSION, on_event, &marker, &core)
          == SAPIE_OK);
    CHECK(core != NULL);

    CHECK(sapie_core_test_connection(core, "not-a-server") == SAPIE_ERR_INVALID_SERVER);
    CHECK(sapie_core_test_connection(NULL, "host:1") == SAPIE_ERR_INVALID_ARGUMENT);
    CHECK(sapie_core_login(core, "not-a-server", "user", "pw", &session)
          == SAPIE_ERR_INVALID_SERVER);
    CHECK(session == NULL);
    CHECK(sapie_core_sign_out(core) == SAPIE_OK);
    CHECK(sapie_core_sign_out(NULL) == SAPIE_ERR_INVALID_ARGUMENT);

    make_game(dir, sizeof(dir), exe, sizeof(exe));
    CHECK(sapie_core_detect_game(core, exe, &detected) == SAPIE_OK);
    CHECK(detected != NULL);
    CHECK(detected->interfaces != NULL && detected->interface_count == 2);
    CHECK(strcmp(detected->arch, "x64") == 0);
    CHECK(strcmp(detected->era, "modern") == 0);
    CHECK(event_count > 0);

    CHECK(sapie_core_library_add(core, exe, &record) == SAPIE_OK);
    CHECK(record != NULL);
    CHECK(strcmp(record->exe_path, exe) == 0);
    CHECK(sapie_core_library_list(core, &records, &count) == SAPIE_OK);
    CHECK(count == 1 && records != NULL);
    CHECK(strcmp(records[0].exe_path, exe) == 0);
    CHECK(records[0].favorite == 0);

    CHECK(sapie_core_set_favorite(core, exe, 1) == SAPIE_OK);
    {
        const char *const args[] = { "-windowed" };
        CHECK(sapie_core_save_game_options(core, exe, args, 1, NULL) == SAPIE_OK);
    }
    CHECK(sapie_core_get_preference(core, "librarySort", "Name", &preference) == SAPIE_OK);
    CHECK(preference != NULL && strcmp(preference, "Name") == 0);
    CHECK(sapie_core_set_preference(core, "librarySort", "Last played") == SAPIE_OK);
    sapie_string_free(preference);
    preference = NULL;
    CHECK(sapie_core_get_preference(core, "librarySort", "Name", &preference) == SAPIE_OK);
    CHECK(preference != NULL && strcmp(preference, "Last played") == 0);
    CHECK(sapie_core_data_folder(core, &folder) == SAPIE_OK);
    CHECK(folder != NULL && folder[0] != '\0');
    CHECK(sapie_core_launcher_version(&version) == SAPIE_OK);
    CHECK(version != NULL && version[0] != '\0');
    CHECK(sapie_core_check_updates(core, "stable", &update) == SAPIE_OK);
    CHECK(update != NULL);
    CHECK(sapie_core_parse_deep_link("sapie://join/123?exp=1", &join_id) == SAPIE_OK);
    CHECK(join_id != NULL && strcmp(join_id, "123") == 0);

    CHECK(sapie_core_install_library(core, exe, &installed) == SAPIE_ERR_NOT_FOUND);
    CHECK(installed == NULL);
    CHECK(sapie_core_restore_library(core, exe, &restored) == SAPIE_OK);
    CHECK(restored != NULL && strstr(restored, "steam_api64.dll") != NULL);
    CHECK(sapie_core_configure_game(core, exe, 480, &configured)
          == SAPIE_ERR_INVALID_SERVER);
    CHECK(configured == NULL);
    CHECK(sapie_core_launch_game(core, exe, &pid) == SAPIE_ERR_TOKEN_EXPIRED);
    CHECK(pid == 99);
    CHECK(sapie_core_unpack_steamstub(core, exe, &unpacked) == SAPIE_ERR_PROCESS);
    CHECK(unpacked == NULL);
    CHECK(sapie_core_library_remove(core, exe) == SAPIE_OK);

    CHECK(sapie_core_server_request(core, "GET", "/friends", NULL, &text)
          == SAPIE_ERR_INVALID_ARGUMENT);
    CHECK(text == NULL);
    CHECK(sapie_core_server_request(core, "GET", "../logout", NULL, &text)
          == SAPIE_ERR_INVALID_ARGUMENT);
    CHECK(sapie_core_revoke_token(core) == SAPIE_OK);

    /* Every owned string and struct has an explicit free path. */
    strings = detected->interfaces;
    sapie_string_array_free(strings, detected->interface_count);
    detected->interfaces = NULL;
    detected->interface_count = 0;
    sapie_string_free(restored);
    restored = NULL;
    sapie_string_free(preference);
    sapie_string_free(folder);
    sapie_string_free(version);
    sapie_string_free(update);
    sapie_string_free(join_id);
    text = NULL;
    sapie_string_free(text);
    sapie_string_free(NULL);
    sapie_string_array_free(NULL, 0);
    sapie_session_info_free(session);
    sapie_detect_result_free(detected);
    sapie_install_result_free(installed);
    sapie_configure_result_free(configured);
    sapie_game_record_free(record);
    sapie_game_record_array_free(records, count);
    sapie_game_record_array_free(NULL, 0);

    sapie_core_destroy(core);
    sapie_core_destroy(NULL);

    if (failures == 0) {
        printf("c_abi_smoke: all exported functions called, %d events\n", event_count);
    }
    return failures == 0 ? 0 : 1;
}
