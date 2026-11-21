#ifndef SAPIE_CORE_H
#define SAPIE_CORE_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/*
 * UI-to-core contract. A caller with a different major version is rejected by
 * sapie_core_create with SAPIE_ERR_ABI_VERSION. Only this header is visible to
 * the Qt side. There is no socket, no JSON command channel and no Qt type here.
 */
#define SAPIE_CORE_ABI_VERSION 1u

typedef struct SapieCore SapieCore;

typedef enum SapieStatus {
    SAPIE_OK = 0,
    SAPIE_ERR_INVALID_ARGUMENT = 1,
    SAPIE_ERR_INVALID_PATH = 2,
    SAPIE_ERR_NOT_FOUND = 3,
    SAPIE_ERR_PERMISSION_DENIED = 4,
    SAPIE_ERR_BACKUP_CONFLICT = 5,
    SAPIE_ERR_INVALID_SERVER = 6,
    SAPIE_ERR_UNREACHABLE = 7,
    SAPIE_ERR_BAD_CREDENTIALS = 8,
    SAPIE_ERR_SERVER_TOO_OLD = 9,
    SAPIE_ERR_SERVER = 10,
    SAPIE_ERR_TOKEN_EXPIRED = 11,
    SAPIE_ERR_KEYCHAIN = 12,
    SAPIE_ERR_IO = 13,
    SAPIE_ERR_PROCESS = 14,
    SAPIE_ERR_ABI_VERSION = 15
} SapieStatus;

/*
 * The single event callback, used for long-operation progress. The two strings
 * belong to the core and are valid only until the callback returns. Copy them
 * if they must outlive the call. user_data is passed back untouched.
 */
typedef void (*SapieEventCallback)(void *user_data, const char *step, const char *message);

typedef struct SapieSessionInfo {
    char *session_token;
    uint64_t steam_id;
    char *persona_name;
    char *server;
    uint64_t expires_at;
} SapieSessionInfo;

typedef struct SapieDetectResult {
    char *exe_path;
    char *arch;
    char *era;
    char **interfaces;
    size_t interface_count;
    char *original_library;
    char *warning;
} SapieDetectResult;

typedef struct SapieInstallResult {
    char *arch;
    char *era;
    char **interfaces;
    size_t interface_count;
    char *original_library;
    char *library;
    char *warning;
} SapieInstallResult;

typedef struct SapieConfigureResult {
    uint32_t appid;
    char *game_server;
    char *note;
} SapieConfigureResult;

typedef struct SapieGameRecord {
    char *exe_path;
    char *arch;
    char *era;
    char **interfaces;
    size_t interface_count;
    char *original_library;
    char *library;
    uint32_t appid;
    int configured;
    char **launch_args;
    size_t launch_arg_count;
    char *ld_preload;
    uint64_t last_played;
} SapieGameRecord;

/* Version and lifetime. */
uint32_t sapie_core_abi_version(void);
SapieStatus sapie_core_create(uint32_t abi_version, SapieEventCallback callback,
                              void *user_data, SapieCore **out_core);
/* Test and fake-core entry point. Keeps the keychain and process spawn in memory. */
SapieStatus sapie_core_create_fake(uint32_t abi_version, SapieEventCallback callback,
                                   void *user_data, SapieCore **out_core);
void sapie_core_destroy(SapieCore *core);

/*
 * Every owned string or struct returned by a command must be released with its
 * matching free function. NULL is safe for all free functions. On a failed
 * command the out parameters are left untouched, so zero them before the call.
 */
void sapie_string_free(char *value);
void sapie_string_array_free(char **values, size_t count);
void sapie_session_info_free(SapieSessionInfo *value);
void sapie_detect_result_free(SapieDetectResult *value);
void sapie_install_result_free(SapieInstallResult *value);
void sapie_configure_result_free(SapieConfigureResult *value);
void sapie_game_record_free(SapieGameRecord *value);
void sapie_game_record_array_free(SapieGameRecord *values, size_t count);

/* Commands. */
SapieStatus sapie_core_test_connection(SapieCore *core, const char *server);
SapieStatus sapie_core_login(SapieCore *core, const char *server, const char *username,
                              const char *password, SapieSessionInfo **out_session);
SapieStatus sapie_core_sign_out(SapieCore *core);
SapieStatus sapie_core_detect_game(SapieCore *core, const char *path,
                                    SapieDetectResult **out_result);
SapieStatus sapie_core_install_library(SapieCore *core, const char *path,
                                        SapieInstallResult **out_result);
SapieStatus sapie_core_restore_library(SapieCore *core, const char *path,
                                        char **out_restored);
SapieStatus sapie_core_configure_game(SapieCore *core, const char *path, uint32_t appid,
                                       SapieConfigureResult **out_result);
SapieStatus sapie_core_launch_game(SapieCore *core, const char *path, uint32_t *out_pid);
SapieStatus sapie_core_unpack_steamstub(SapieCore *core, const char *path,
                                         char **out_unpacked);
SapieStatus sapie_core_library_list(SapieCore *core, SapieGameRecord **out_records,
                                     size_t *out_count);
SapieStatus sapie_core_library_add(SapieCore *core, const char *path,
                                    SapieGameRecord **out_record);
SapieStatus sapie_core_library_remove(SapieCore *core, const char *path);

#ifdef __cplusplus
}
#endif

#endif
