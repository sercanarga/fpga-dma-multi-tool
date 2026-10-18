cmake_minimum_required(VERSION 3.24)
project(FpgaDmaMultiTool VERSION 3.0.0 LANGUAGES CXX)
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
include(CTest)
option(FPGA_SANITIZERS "Enable address and undefined behavior sanitizers" OFF)
function(fpga_target target)
  target_include_directories(${target} PUBLIC src)
  target_compile_features(${target} PUBLIC cxx_std_20)
  if(MSVC)
    target_compile_options(${target} PRIVATE /W4 /WX /permissive- /utf-8 /EHsc)
  else()
    target_compile_options(${target} PRIVATE -Wall -Wextra -Wpedantic -Werror)
  endif()
endfunction()
if(FPGA_SANITIZERS AND NOT WIN32)
  add_compile_options(-fsanitize=address,undefined -fno-omit-frame-pointer)
  add_link_options(-fsanitize=address,undefined)
endif()
add_library(fpga_core STATIC
  src/core/files.cpp
  src/core/text.cpp
  src/core/catalog.cpp
  src/core/jtag.cpp
  src/core/firmware.cpp
  src/core/benchmark.cpp
  src/core/inspection.cpp
)
fpga_target(fpga_core)
add_library(fpga_application STATIC
  src/application/model.cpp
  src/application/protocol.cpp
  src/application/event_stream.cpp
  src/application/history.cpp
)
target_include_directories(fpga_application SYSTEM PRIVATE third_party)
target_link_libraries(fpga_application PUBLIC fpga_core)
fpga_target(fpga_application)
if(WIN32)
  enable_language(RC)
  add_compile_definitions(UNICODE _UNICODE NOMINMAX WIN32_LEAN_AND_MEAN _WIN32_WINNT=0x0A00)
  add_library(fpga_platform STATIC
    src/platform/windows.cpp
    src/platform/process.cpp
    src/platform/socket.cpp
    src/platform/ch347.cpp
    src/platform/timer.cpp
    src/platform/writer.cpp
    src/platform/programming.cpp
    src/platform/benchmark.cpp
    src/platform/leechcore.cpp
    src/platform/devices.cpp
    src/platform/registry.cpp
    src/platform/system_info.cpp
    src/platform/drivers.cpp
    src/platform/administration.cpp
    src/platform/service.cpp
    src/application/worker.cpp
  )
  target_include_directories(fpga_platform SYSTEM PRIVATE third_party third_party/leechcore)
  if(MINGW AND NOT CMAKE_HOST_WIN32)
    target_include_directories(fpga_platform SYSTEM PRIVATE cmake/compat)
  endif()
  target_link_libraries(fpga_platform PUBLIC
    fpga_application
    ws2_32
    ole32
    shell32
    bcrypt
    advapi32
    uuid
    setupapi
    cfgmgr32
    wbemuuid
    oleaut32
    wintrust
    crypt32
  )
  fpga_target(fpga_platform)
  add_library(fpga_ui STATIC
    src/application/work_queue.cpp
    src/ui/controls.cpp
    src/ui/table.cpp
    src/ui/page.cpp
    src/ui/devices_page.cpp
    src/ui/firmware_page.cpp
    src/ui/benchmark_page.cpp
    src/ui/history_page.cpp
    src/ui/system_page.cpp
    src/ui/drivers_page.cpp
    src/ui/details.cpp
    src/ui/window.cpp
    src/ui/window_operations.cpp
    src/ui/window_view.cpp
    src/ui/settings.cpp
  )
  target_link_libraries(fpga_ui PUBLIC fpga_platform comctl32 gdi32 user32)
  target_compile_definitions(fpga_ui PRIVATE FPGA_VERSION_W=L"${PROJECT_VERSION}")
  fpga_target(fpga_ui)
  configure_file(packaging/windows/app.manifest.in generated/app.manifest @ONLY)
  configure_file(packaging/windows/app.rc.in generated/app.rc @ONLY)
  set_property(SOURCE "${CMAKE_CURRENT_BINARY_DIR}/generated/app.rc" PROPERTY OBJECT_DEPENDS "${CMAKE_CURRENT_SOURCE_DIR}/assets/branding/app.ico")
  add_executable(FPGA-DMA-Multi-Tool WIN32 src/main.cpp "${CMAKE_CURRENT_BINARY_DIR}/generated/app.rc")
  target_link_libraries(FPGA-DMA-Multi-Tool PRIVATE fpga_ui)
  target_link_options(FPGA-DMA-Multi-Tool PRIVATE $<$<CXX_COMPILER_ID:MSVC>:/MANIFEST:NO>)
  fpga_target(FPGA-DMA-Multi-Tool)
  add_executable(cygpath tools/cygpath.cpp)
  fpga_target(cygpath)
  if(MINGW)
    foreach(target FPGA-DMA-Multi-Tool cygpath)
      target_link_options(${target} PRIVATE -municode -static $<$<CONFIG:Release>:-s>)
    endforeach()
  endif()
  install(TARGETS FPGA-DMA-Multi-Tool RUNTIME DESTINATION .)
  install(TARGETS cygpath RUNTIME DESTINATION openFPGALoader)
  install(DIRECTORY packaging/windows/runtime/ DESTINATION .)
  install(FILES LICENSE.md DESTINATION licenses RENAME APPLICATION.txt)
  install(FILES third_party/leechcore/LICENSE.txt DESTINATION licenses RENAME LEECHCORE-AGPL-3.0.txt)
  install(FILES third_party/nlohmann/LICENSE.MIT DESTINATION licenses RENAME NLOHMANN-JSON-MIT.txt)
  set(CPACK_GENERATOR ZIP)
  set(CPACK_PACKAGE_FILE_NAME FPGA-DMA-Multi-Tool-Windows-x64)
  set(CPACK_PACKAGE_DIRECTORY "${CMAKE_BINARY_DIR}/package")
  set(CPACK_INCLUDE_TOPLEVEL_DIRECTORY OFF)
  include(CPack)
endif()
if(BUILD_TESTING)
  add_executable(core_tests tests/core_tests.cpp)
  target_link_libraries(core_tests PRIVATE fpga_application)
  fpga_target(core_tests)
  add_test(NAME core COMMAND core_tests)
  add_executable(application_tests tests/application_tests.cpp)
  target_link_libraries(application_tests PRIVATE fpga_application)
  fpga_target(application_tests)
  add_test(NAME application COMMAND application_tests)
  if(WIN32)
    add_executable(transport_tests tests/transport_tests.cpp)
    target_link_libraries(transport_tests PRIVATE fpga_platform)
    fpga_target(transport_tests)
    if(MINGW)
      target_link_options(transport_tests PRIVATE -static $<$<CONFIG:Release>:-s>)
    endif()
    add_test(NAME transport COMMAND transport_tests)
    add_executable(platform_tests tests/platform_tests.cpp "${CMAKE_CURRENT_BINARY_DIR}/generated/app.rc")
    target_link_libraries(platform_tests PRIVATE fpga_ui)
    target_link_options(platform_tests PRIVATE $<$<CXX_COMPILER_ID:MSVC>:/MANIFEST:NO>)
    target_include_directories(platform_tests SYSTEM PRIVATE third_party/leechcore)
    if(MINGW AND NOT CMAKE_HOST_WIN32)
      target_include_directories(platform_tests SYSTEM PRIVATE cmake/compat)
    endif()
    fpga_target(platform_tests)
    if(MINGW)
      target_link_options(core_tests PRIVATE -static $<$<CONFIG:Release>:-s>)
      target_link_options(application_tests PRIVATE -static $<$<CONFIG:Release>:-s>)
      target_link_options(platform_tests PRIVATE -static $<$<CONFIG:Release>:-s>)
    endif()
    add_test(NAME platform COMMAND platform_tests)
    install(TARGETS core_tests application_tests platform_tests transport_tests
      RUNTIME DESTINATION . COMPONENT Tests EXCLUDE_FROM_ALL)
    add_executable(documentation_screenshots EXCLUDE_FROM_ALL tools/screenshots.cpp "${CMAKE_CURRENT_BINARY_DIR}/generated/app.rc")
    target_link_libraries(documentation_screenshots PRIVATE fpga_ui windowscodecs dwmapi)
    target_link_options(documentation_screenshots PRIVATE $<$<CXX_COMPILER_ID:MSVC>:/MANIFEST:NO>)
    fpga_target(documentation_screenshots)
    if(MINGW)
      target_link_options(documentation_screenshots PRIVATE -municode -static $<$<CONFIG:Release>:-s>)
    endif()
  endif()
endif()
