cmake_minimum_required(VERSION 3.21)

project(sapie_launcher LANGUAGES C CXX)

set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

option(SAPIE_CARGO_OFFLINE "Build the Rust core without network access" OFF)

find_package(Qt6 6.8 REQUIRED COMPONENTS Core Gui Qml Quick Network Test)
find_package(Threads REQUIRED)

qt_standard_project_setup()

set(SAPIE_CARGO_TARGET_DIR "${CMAKE_BINARY_DIR}/cargo-target")
if(WIN32 AND MSVC)
  set(SAPIE_CORE_STATIC "${SAPIE_CARGO_TARGET_DIR}/release/sapie_core.lib")
else()
  set(SAPIE_CORE_STATIC "${SAPIE_CARGO_TARGET_DIR}/release/libsapie_core.a")
endif()

file(GLOB_RECURSE SAPIE_CORE_SOURCES CONFIGURE_DEPENDS
  "${CMAKE_CURRENT_SOURCE_DIR}/core/src/*.rs"
  "${CMAKE_CURRENT_SOURCE_DIR}/core/Cargo.toml"
  "${CMAKE_CURRENT_SOURCE_DIR}/Cargo.toml"
  "${CMAKE_CURRENT_SOURCE_DIR}/Cargo.lock"
)

set(SAPIE_CARGO_ARGS build --release --manifest-path "${CMAKE_CURRENT_SOURCE_DIR}/Cargo.toml")
if(SAPIE_CARGO_OFFLINE)
  list(APPEND SAPIE_CARGO_ARGS --offline)
endif()

add_custom_command(
  OUTPUT "${SAPIE_CORE_STATIC}"
  COMMAND "${CMAKE_COMMAND}" -E env "CARGO_TARGET_DIR=${SAPIE_CARGO_TARGET_DIR}" cargo ${SAPIE_CARGO_ARGS}
  DEPENDS ${SAPIE_CORE_SOURCES}
  WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
  COMMENT "Building the SAPIE Rust core"
  VERBATIM
)
add_custom_target(sapie_core_build DEPENDS "${SAPIE_CORE_STATIC}")

add_library(sapie_core STATIC IMPORTED GLOBAL)
set_target_properties(sapie_core PROPERTIES
  IMPORTED_LOCATION "${SAPIE_CORE_STATIC}"
  INTERFACE_INCLUDE_DIRECTORIES "${CMAKE_CURRENT_SOURCE_DIR}/core/include"
)
target_link_libraries(sapie_core INTERFACE Threads::Threads)
if(UNIX AND NOT APPLE)
  find_library(SAPIE_DBUS_LIBRARY NAMES dbus-1 REQUIRED)
  target_link_libraries(sapie_core INTERFACE "${CMAKE_DL_LIBS}" m "${SAPIE_DBUS_LIBRARY}")
elseif(WIN32)
  target_link_libraries(sapie_core INTERFACE advapi32 crypt32 user32)
endif()

add_library(sapie_bridge STATIC
  bridge/CoreBridge.cpp
  bridge/CoreBridge.h
)
target_include_directories(sapie_bridge PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}/bridge")
target_link_libraries(sapie_bridge PUBLIC Qt6::Core sapie_core)
add_dependencies(sapie_bridge sapie_core_build)

qt_add_executable(sapie_launcher
  app/main.cpp
  app/main.qml
  app/app.qrc
)
target_link_libraries(sapie_launcher PRIVATE Qt6::Core Qt6::Gui Qt6::Qml Qt6::Quick sapie_bridge)
add_dependencies(sapie_launcher sapie_core_build)

include(CTest)
enable_testing()

add_executable(sapie_c_abi_smoke tests/c_abi_smoke.c)
set_target_properties(sapie_c_abi_smoke PROPERTIES LINKER_LANGUAGE CXX)
target_include_directories(sapie_c_abi_smoke PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}/core/include")
target_link_libraries(sapie_c_abi_smoke PRIVATE sapie_core)
add_dependencies(sapie_c_abi_smoke sapie_core_build)
add_test(NAME c_abi_smoke COMMAND sapie_c_abi_smoke)

qt_add_executable(core_bridge_test
  tests/core_bridge_test.cpp
  tests/core_bridge_test.h
  tests/FakePlatform.h
)
target_include_directories(core_bridge_test PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}/app")
target_link_libraries(core_bridge_test PRIVATE Qt6::Core Qt6::Network Qt6::Test sapie_bridge)
add_dependencies(core_bridge_test sapie_core_build)
add_test(NAME core_bridge_test COMMAND core_bridge_test)
