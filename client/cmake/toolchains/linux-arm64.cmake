set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR aarch64)

set(CMAKE_C_COMPILER aarch64-linux-gnu-gcc)
set(CMAKE_CXX_COMPILER aarch64-linux-gnu-g++)

set(CMAKE_FIND_ROOT_PATH /usr/aarch64-linux-gnu)
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE NEVER)

# Prefer a static C++ runtime so the .so only depends on the cross protobuf-lite.
set(CMAKE_SHARED_LINKER_FLAGS "-static-libgcc -static-libstdc++" CACHE STRING "" FORCE)
set(CMAKE_EXE_LINKER_FLAGS "-static-libgcc -static-libstdc++" CACHE STRING "" FORCE)

get_filename_component(SAPIE_REPO_ROOT "${CMAKE_CURRENT_LIST_DIR}/../.." ABSOLUTE)
set(SAPIE_PROTOBUF_ROOT "${SAPIE_REPO_ROOT}/third_party/protobuf/linux-arm64" CACHE PATH "Cross-built protobuf-lite prefix")
