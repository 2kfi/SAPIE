if(CMAKE_SOURCE_DIR STREQUAL CMAKE_CURRENT_SOURCE_DIR)
  cmake_minimum_required(VERSION 3.24)
  project(sapie-linux LANGUAGES CXX)
  enable_testing()
endif()

find_package(Qt6 6.8 REQUIRED COMPONENTS Core Gui Network DBus Test)

qt_standard_project_setup()

get_filename_component(SAPIE_LAUNCHER_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/.." ABSOLUTE)
if(NOT DEFINED SAPIE_REPO_ROOT)
  get_filename_component(SAPIE_REPO_ROOT "${SAPIE_LAUNCHER_ROOT}/.." ABSOLUTE)
endif()

set(SAPIE_LINUX_INCLUDE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/include")
set(SAPIE_LINUX_PACKAGING_DIR "${CMAKE_CURRENT_SOURCE_DIR}/packaging")
set(SAPIE_LINUX_RESOURCE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/resources")
set(SAPIE_LINUX_OUTPUT_DIR "${CMAKE_CURRENT_BINARY_DIR}/resources")
set(SAPIE_LINUX_DESKTOP_ID "org.sapie.SAPIE.desktop")

if(NOT DEFINED SAPIE_VERSION)
  set(SAPIE_VERSION "0.5.0")
endif()
string(TIMESTAMP SAPIE_RELEASE_DATE "%Y-%m-%d" UTC)
if(NOT DEFINED SAPIE_ARCH)
  if(CMAKE_SYSTEM_PROCESSOR MATCHES "^(x86_64|amd64|AMD64)$")
    set(SAPIE_ARCH "x64")
  elseif(CMAKE_SYSTEM_PROCESSOR MATCHES "^(aarch64|arm64|ARM64)$")
    set(SAPIE_ARCH "arm64")
  else()
    message(FATAL_ERROR "Unsupported Linux architecture: ${CMAKE_SYSTEM_PROCESSOR}")
  endif()
endif()
if(NOT DEFINED SAPIE_LINUX_EXE)
  set(SAPIE_LINUX_EXE "${CMAKE_BINARY_DIR}/sapie-launcher")
endif()
if(NOT DEFINED SAPIE_FLATPAK_REPO)
  set(SAPIE_FLATPAK_REPO "${CMAKE_CURRENT_BINARY_DIR}/flatpak-repo")
endif()

file(MAKE_DIRECTORY "${SAPIE_LINUX_OUTPUT_DIR}")
configure_file(
  "${SAPIE_LINUX_RESOURCE_DIR}/org.sapie.SAPIE.desktop.in"
  "${SAPIE_LINUX_OUTPUT_DIR}/${SAPIE_LINUX_DESKTOP_ID}"
  @ONLY
)
configure_file(
  "${SAPIE_LINUX_RESOURCE_DIR}/org.sapie.SAPIE.metainfo.xml.in"
  "${SAPIE_LINUX_OUTPUT_DIR}/org.sapie.SAPIE.metainfo.xml"
  @ONLY
)

qt_add_library(sapie-linux-platform STATIC
  src/linux_native.cpp
  src/linux_token.cpp
  src/linux_updater.cpp
  src/linux_platform.cpp
  include/linux_native.h
  include/linux_token.h
  include/linux_updater.h
  include/linux_platform.h
)
set_target_properties(sapie-linux-platform PROPERTIES AUTOMOC ON)
target_include_directories(sapie-linux-platform PUBLIC "${SAPIE_LINUX_INCLUDE_DIR}")
target_link_libraries(sapie-linux-platform
  PUBLIC Qt6::Core Qt6::Gui Qt6::Network Qt6::DBus
)
target_compile_definitions(sapie-linux-platform PUBLIC
  SAPIE_DESKTOP_ID="${SAPIE_LINUX_DESKTOP_ID}"
)

qt_add_executable(sapie-linux-selfcheck MANUAL_FINALIZATION
  tests/linux_selfcheck.cpp
)
target_link_libraries(sapie-linux-selfcheck PRIVATE sapie-linux-platform Qt6::Test)
add_test(NAME sapie-linux-selfcheck COMMAND sapie-linux-selfcheck)
set_tests_properties(sapie-linux-selfcheck PROPERTIES
  ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
)
qt_finalize_executable(sapie-linux-selfcheck)

find_program(SAPIE_RSVG_CONVERT NAMES rsvg-convert)
find_program(SAPIE_IMAGEMAGICK NAMES magick convert)
find_program(SAPIE_INKSCAPE NAMES inkscape)
if(SAPIE_RSVG_CONVERT OR SAPIE_IMAGEMAGICK OR SAPIE_INKSCAPE)
  add_custom_target(sapie-linux-icons
    COMMAND "${CMAKE_CURRENT_SOURCE_DIR}/packaging/make-icons.sh"
            "${SAPIE_REPO_ROOT}" "${CMAKE_CURRENT_BINARY_DIR}/hicolor"
    COMMENT "Generate hicolor icons from the provisional SVG sources"
    VERBATIM
  )
else()
  message(STATUS "No SVG renderer found: sapie-linux-icons target is not registered")
endif()

find_program(SAPIE_LINUXDEPLOY NAMES linuxdeploy)
find_program(SAPIE_APPIMAGETOOL NAMES appimagetool)
if(SAPIE_LINUXDEPLOY AND SAPIE_APPIMAGETOOL)
  add_custom_target(sapie-linux-appimage
    COMMAND "${SAPIE_LINUX_PACKAGING_DIR}/build-appimage.sh"
            "${SAPIE_LINUX_EXE}" "${SAPIE_LINUX_OUTPUT_DIR}"
            "${CMAKE_CURRENT_BINARY_DIR}/hicolor"
            "${CMAKE_CURRENT_BINARY_DIR}/appimage" "${SAPIE_ARCH}" "${SAPIE_VERSION}"
    COMMENT "Build the SAPIE AppImage with linuxdeploy and its Qt plugin"
    VERBATIM
  )
else()
  message(STATUS "linuxdeploy or appimagetool is missing: sapie-linux-appimage is not registered")
endif()

find_program(SAPIE_DPKG_DEB NAMES dpkg-deb)
if(SAPIE_DPKG_DEB)
  add_custom_target(sapie-linux-deb
    COMMAND "${SAPIE_LINUX_PACKAGING_DIR}/build-deb.sh"
            "${SAPIE_LINUX_EXE}" "${SAPIE_LINUX_OUTPUT_DIR}"
            "${CMAKE_CURRENT_BINARY_DIR}/hicolor"
            "${CMAKE_CURRENT_BINARY_DIR}/deb" "${SAPIE_VERSION}" "${SAPIE_ARCH}"
    COMMENT "Build the SAPIE deb with a private Qt tree under /opt/SAPIE"
    VERBATIM
  )
else()
  message(STATUS "dpkg-deb is missing: sapie-linux-deb is not registered")
endif()

find_program(SAPIE_FLATPAK_BUILDER NAMES flatpak-builder)
if(SAPIE_FLATPAK_BUILDER)
  add_custom_target(sapie-linux-flatpak
    COMMAND "${SAPIE_LINUX_PACKAGING_DIR}/build-flatpak.sh"
            "${SAPIE_REPO_ROOT}" "${CMAKE_CURRENT_BINARY_DIR}/flatpak"
            "${SAPIE_ARCH}" "${SAPIE_VERSION}" "${SAPIE_FLATPAK_REPO}"
    COMMENT "Build the SAPIE Flatpak on org.kde.Platform"
    VERBATIM
  )
else()
  message(STATUS "flatpak-builder is missing: sapie-linux-flatpak is not registered")
endif()

find_package(Python3 QUIET COMPONENTS Interpreter)
if(Python3_Interpreter_FOUND)
  add_custom_target(sapie-linux-check-manifest
    COMMAND "${CMAKE_CURRENT_SOURCE_DIR}/scripts/check-update-manifest.sh"
            "\${SAPIE_MANIFEST}" "\${SAPIE_PUBLIC_KEY}" stable "\${SAPIE_VERSION}"
    COMMENT "Verify a signed Linux update manifest with tools/update-manifest.py"
    VERBATIM
  )
endif()
