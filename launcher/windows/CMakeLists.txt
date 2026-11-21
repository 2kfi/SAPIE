if(CMAKE_SOURCE_DIR STREQUAL CMAKE_CURRENT_SOURCE_DIR)
  cmake_minimum_required(VERSION 3.24)
  project(sapie-windows LANGUAGES CXX)
  enable_testing()
endif()

find_package(Qt6 6.8 REQUIRED COMPONENTS Core Gui Widgets Network)

qt_standard_project_setup()

set(SAPIE_WINDOWS_INCLUDE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/include")
set(SAPIE_WINDOWS_RESOURCE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/resources")
set(SAPIE_WINDOWS_PACKAGING_DIR "${CMAKE_CURRENT_SOURCE_DIR}/packaging")
set(SAPIE_WINDOWS_SIGNING_DIR "${CMAKE_CURRENT_SOURCE_DIR}/signing")

if(NOT DEFINED SAPIE_VERSION)
  set(SAPIE_VERSION "0.5.0")
endif()
if(NOT DEFINED SAPIE_VERSION_RC)
  string(REPLACE "." "," SAPIE_VERSION_RC "${SAPIE_VERSION}")
  set(SAPIE_VERSION_RC "${SAPIE_VERSION_RC},0")
endif()
if(NOT DEFINED SAPIE_WINDOWS_EXE)
  set(SAPIE_WINDOWS_EXE "${CMAKE_BINARY_DIR}/sapie-launcher.exe")
endif()

if(WIN32)
  configure_file(
    "${SAPIE_WINDOWS_RESOURCE_DIR}/app.rc.in"
    "${CMAKE_CURRENT_BINARY_DIR}/app.rc"
    @ONLY
  )
  configure_file(
    "${SAPIE_WINDOWS_RESOURCE_DIR}/qt.conf.in"
    "${CMAKE_CURRENT_BINARY_DIR}/qt.conf"
    COPYONLY
  )
  configure_file(
    "${SAPIE_WINDOWS_RESOURCE_DIR}/sapie.exe.manifest"
    "${CMAKE_CURRENT_BINARY_DIR}/sapie.exe.manifest"
    COPYONLY
  )

  set(SAPIE_WINDOWS_SOURCES
    src/win_native.cpp
    src/win_platform.cpp
    src/win_acl.cpp
    src/win_steamless.cpp
    src/win_updater.cpp
  )

  qt_add_library(sapie-windows-platform STATIC
    ${SAPIE_WINDOWS_SOURCES}
    "${CMAKE_CURRENT_BINARY_DIR}/app.rc"
  )

  set_target_properties(sapie-windows-platform PROPERTIES AUTOMOC ON)

  target_include_directories(sapie-windows-platform
    PUBLIC "${SAPIE_WINDOWS_INCLUDE_DIR}"
    PRIVATE "${CMAKE_CURRENT_BINARY_DIR}"
  )

  target_compile_definitions(sapie-windows-platform
    PRIVATE UNICODE _UNICODE WIN32_LEAN_AND_MEAN NOMINMAX
  )

  target_link_libraries(sapie-windows-platform
    PUBLIC Qt6::Core Qt6::Gui Qt6::Widgets Qt6::Network
    PRIVATE dwmapi shell32 ole32 crypt32 advapi32 bcrypt ncrypt wintrust propsys
  )

  add_executable(sapie-windows-selfcheck tests/win_selfcheck.cpp)
  target_link_libraries(sapie-windows-selfcheck PRIVATE sapie-windows-platform Qt6::Core)
  add_test(NAME sapie-windows-selfcheck COMMAND sapie-windows-selfcheck)

  find_program(SAPIE_POWERSHELL powershell.exe powershell pwsh)
  if(SAPIE_POWERSHELL)
    add_custom_target(sapie-windows-icons
      COMMAND "${SAPIE_POWERSHELL}" -NoProfile -ExecutionPolicy Bypass
              -File "${SAPIE_WINDOWS_PACKAGING_DIR}/make-icons.ps1"
              -RepoRoot "${CMAKE_SOURCE_DIR}"
              -OutputDir "${CMAKE_BINARY_DIR}/sapie-icons"
      COMMENT "Generate Windows icon resources from the provisional SVG sources"
      VERBATIM
    )

    add_custom_target(sapie-windows-deploy
      COMMAND "${SAPIE_POWERSHELL}" -NoProfile -ExecutionPolicy Bypass
              -File "${SAPIE_WINDOWS_PACKAGING_DIR}/deploy-qt.ps1"
              -QtBin "${QT6_INSTALL_PREFIX}/${QT6_INSTALL_BINS}"
              -Exe "${SAPIE_WINDOWS_EXE}"
              -InstallDir "${CMAKE_BINARY_DIR}/sapie-stage"
              -RepoRoot "${CMAKE_SOURCE_DIR}"
      COMMENT "Deploy Qt runtime and Steamless into the portable stage"
      VERBATIM
    )

    add_custom_target(sapie-windows-package
      COMMAND "${CMAKE_COMMAND}" -E echo
              "Run packaging/build-package.ps1 with a signed binary and a version"
    )
  else()
    message(STATUS "PowerShell not found: packaging targets are not registered")
  endif()
else()
  message(STATUS "SAPIE Windows platform sources are skipped on this host")
endif()
