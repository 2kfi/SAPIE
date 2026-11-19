cmake_minimum_required(VERSION 3.16)
project(sapie_client LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

set(SAPIE_INTERFACE_ERA "legacy" CACHE STRING "Steam interface era: legacy or modern")
set_property(CACHE SAPIE_INTERFACE_ERA PROPERTY STRINGS legacy modern)
if(NOT SAPIE_INTERFACE_ERA MATCHES "^(legacy|modern)$")
  message(FATAL_ERROR "SAPIE_INTERFACE_ERA must be 'legacy' or 'modern', got '${SAPIE_INTERFACE_ERA}'")
endif()

if(NOT CMAKE_LIBRARY_OUTPUT_DIRECTORY)
  set(CMAKE_LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}")
endif()
# On Windows a shared library's DLL is a runtime artifact, keep it beside the .so output.
if(NOT CMAKE_RUNTIME_OUTPUT_DIRECTORY)
  set(CMAKE_RUNTIME_OUTPUT_DIRECTORY "${CMAKE_LIBRARY_OUTPUT_DIRECTORY}")
endif()

find_program(SAPIE_PROTOC NAMES protoc)
if(NOT SAPIE_PROTOC)
  message(FATAL_ERROR "protoc not found; install protobuf (pacman -S protobuf)")
endif()
if(SAPIE_PROTOBUF_ROOT)
  # Never fall back to /usr/include on a cross target: those headers pull in
  # x86_64 Linux system headers and break the mingw/cross compilers.
  set(SAPIE_PROTOBUF_INCLUDE_DIR "")
  if(EXISTS "${SAPIE_PROTOBUF_ROOT}/include/google/protobuf/message_lite.h")
    set(SAPIE_PROTOBUF_INCLUDE_DIR "${SAPIE_PROTOBUF_ROOT}/include")
  else()
    message(STATUS "Cross protobuf-lite headers missing; run tools/build-protobuf.sh for this target")
  endif()
else()
  find_path(SAPIE_PROTOBUF_INCLUDE_DIR NAMES google/protobuf/message_lite.h)
  if(NOT SAPIE_PROTOBUF_INCLUDE_DIR)
    message(FATAL_ERROR "protobuf headers not found; install protobuf (pacman -S protobuf)")
  endif()
endif()

get_filename_component(SAPIE_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../proto" REALPATH)
set(SAPIE_PROTO_GEN_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")
file(MAKE_DIRECTORY "${SAPIE_PROTO_GEN_DIR}")
set(SAPIE_PROTO_SRCS
  "${SAPIE_PROTO_GEN_DIR}/net.pb.cc"
  "${SAPIE_PROTO_GEN_DIR}/sapie.pb.cc")
set(SAPIE_PROTO_HDRS
  "${SAPIE_PROTO_GEN_DIR}/net.pb.h"
  "${SAPIE_PROTO_GEN_DIR}/sapie.pb.h")
add_custom_command(
  OUTPUT ${SAPIE_PROTO_SRCS} ${SAPIE_PROTO_HDRS}
  COMMAND ${SAPIE_PROTOC}
          --proto_path=${SAPIE_PROTO_DIR}
          --cpp_out=${SAPIE_PROTO_GEN_DIR}
          ${SAPIE_PROTO_DIR}/net.proto ${SAPIE_PROTO_DIR}/sapie.proto
  DEPENDS "${SAPIE_PROTO_DIR}/net.proto" "${SAPIE_PROTO_DIR}/sapie.proto"
  COMMENT "Generating net.pb.* and sapie.pb.*"
  VERBATIM)

file(GLOB SAPIE_DLL_SRC CONFIGURE_DEPENDS
  dll/*.cpp dll/*.h)

add_library(steam_api SHARED ${SAPIE_DLL_SRC} ${SAPIE_PROTO_SRCS} ${SAPIE_PROTO_HDRS})

target_include_directories(steam_api PRIVATE
  "${SAPIE_PROTO_GEN_DIR}"
  "${CMAKE_CURRENT_SOURCE_DIR}")
if(SAPIE_PROTOBUF_INCLUDE_DIR)
  target_include_directories(steam_api PRIVATE "${SAPIE_PROTOBUF_INCLUDE_DIR}")
endif()

# Cross builds use the static lib installed by tools/build-protobuf.sh. Its
# exported CMake target carries the absl/utf8_range deps that plain
# -lprotobuf-lite would omit.
if(SAPIE_PROTOBUF_ROOT)
  list(PREPEND CMAKE_PREFIX_PATH "${SAPIE_PROTOBUF_ROOT}")
  find_package(Protobuf CONFIG QUIET)
endif()
if(NOT TARGET protobuf::libprotobuf-lite AND NOT CMAKE_CROSSCOMPILING)
  find_package(Protobuf QUIET)
endif()
if(WIN32 AND TARGET absl::base)
  # absl's exported target bakes the host's librt path into its interface libs;
  # MinGW has no rt library and the flag is not needed on Windows.
  get_target_property(_sapie_absl_base_libs absl::base INTERFACE_LINK_LIBRARIES)
  string(REGEX REPLACE "\\$<LINK_ONLY:[^;]*-lrt[^;]*>" "" _sapie_absl_base_libs "${_sapie_absl_base_libs}")
  set_property(TARGET absl::base PROPERTY INTERFACE_LINK_LIBRARIES "${_sapie_absl_base_libs}")
endif()
if(TARGET protobuf::libprotobuf-lite)
  set(SAPIE_PROTOBUF_LIBRARY protobuf::libprotobuf-lite)
else()
  set(SAPIE_PROTOBUF_LIBRARY protobuf-lite)
  if(SAPIE_PROTOBUF_ROOT)
    target_link_directories(steam_api PRIVATE "${SAPIE_PROTOBUF_ROOT}/lib")
  endif()
endif()

find_package(Threads REQUIRED)
target_link_libraries(steam_api PRIVATE
  ${SAPIE_PROTOBUF_LIBRARY}
  Threads::Threads
  ${CMAKE_DL_LIBS})

if(WIN32)
  # MinGW + the Steamworks headers otherwise switch to void-return virtuals
  # that Goldberg's implementations do not override.
  target_compile_definitions(steam_api PRIVATE _S4N_)
  target_link_libraries(steam_api PRIVATE ws2_32 iphlpapi)
  if(CMAKE_SIZEOF_VOID_P EQUAL 8)
    set(SAPIE_DLL_NAME steam_api64)
  else()
    set(SAPIE_DLL_NAME steam_api)
  endif()
  set_target_properties(steam_api PROPERTIES OUTPUT_NAME "${SAPIE_DLL_NAME}" PREFIX "")
else()
  set_target_properties(steam_api PROPERTIES OUTPUT_NAME steam_api PREFIX "lib" SUFFIX ".so")
endif()

target_compile_definitions(steam_api PRIVATE
  $<$<NOT:$<CONFIG:Debug>>:EMU_RELEASE_BUILD>)
if(SAPIE_INTERFACE_ERA STREQUAL "legacy")
  target_compile_definitions(steam_api PRIVATE SAPIE_LEGACY_INTERFACES)
endif()

if(UNIX AND NOT CMAKE_CROSSCOMPILING)
  add_executable(sapie_login_smoke harness/sapie_login_smoke.cpp)
  target_include_directories(sapie_login_smoke PRIVATE
    "${SAPIE_PROTO_GEN_DIR}"
    "${CMAKE_CURRENT_SOURCE_DIR}/sdk_includes")
  if(SAPIE_PROTOBUF_INCLUDE_DIR)
    target_include_directories(sapie_login_smoke PRIVATE "${SAPIE_PROTOBUF_INCLUDE_DIR}")
  endif()
  target_link_libraries(sapie_login_smoke PRIVATE steam_api ${SAPIE_PROTOBUF_LIBRARY} Threads::Threads)
  set_target_properties(sapie_login_smoke PROPERTIES CXX_VISIBILITY_PRESET hidden)
endif()

install(TARGETS steam_api
  RUNTIME DESTINATION .
  LIBRARY DESTINATION .)
