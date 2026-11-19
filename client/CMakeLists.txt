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

# Native builds use the system protobuf-lite CMake target; cross builds use the
# static lib installed by tools/build-protobuf.sh (see SAPIE_PROTOBUF_ROOT).
if(NOT CMAKE_CROSSCOMPILING)
  find_package(Protobuf QUIET)
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

install(TARGETS steam_api
  RUNTIME DESTINATION .
  LIBRARY DESTINATION .)
