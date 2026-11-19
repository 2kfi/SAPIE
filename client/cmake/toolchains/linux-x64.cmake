# Native x86_64 host build: no CMAKE_SYSTEM_NAME so CMake still finds the
# system protobuf-lite CMake target instead of the cross-built static library.
set(CMAKE_C_COMPILER gcc)
set(CMAKE_CXX_COMPILER g++)

get_filename_component(SAPIE_REPO_ROOT "${CMAKE_CURRENT_LIST_DIR}/../../.." ABSOLUTE)
set(SAPIE_PROTOBUF_ROOT "" CACHE PATH "Cross-built protobuf-lite prefix (unused on the host)")
