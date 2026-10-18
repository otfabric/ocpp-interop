# SPDX-License-Identifier: MIT
#
# The ocpp-interop adapter for EVerest libocpp: one executable linked against
# libocpp and the EVerest libraries it needs (log, timer, sqlite,
# evse_security), all built from the pinned everest-core tree. Nothing else of
# everest-core is built. The adapter's Dockerfile drives this build.

cmake_minimum_required(VERSION 3.16)

project(ocpp-interop-everest
    DESCRIPTION "ocpp-interop adapter for EVerest libocpp"
    LANGUAGES CXX C
)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# lib/everest of the pinned everest-core tree.
set(EVEREST_LIB_DIR "" CACHE PATH "lib/everest of the everest-core tree")
if(NOT EXISTS "${EVEREST_LIB_DIR}/ocpp/CMakeLists.txt")
    message(FATAL_ERROR "EVEREST_LIB_DIR must point to lib/everest of everest-core")
endif()

# Dependencies come from the system and from the pinned sources the
# Dockerfile builds, never from EVerest's dependency manager (EDM), which
# would fetch them at configure time.
set(DISABLE_EDM ON CACHE BOOL "" FORCE)
set(BUILD_TESTING OFF CACHE BOOL "" FORCE)
set(LIBOCPP16_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(Boost_USE_STATIC_LIBS ON)

find_package(everest-cmake 0.5 REQUIRED)

# Policy as in libocpp: FindBoost.
if(POLICY CMP0167)
    cmake_policy(SET CMP0167 OLD)
endif()
find_package(Threads REQUIRED)
find_package(Boost COMPONENTS filesystem log log_setup program_options regex system thread REQUIRED)

# libocpp links the target name of json-schema-validator's in-tree build;
# its installed package exports nlohmann_json_schema_validator::validator.
find_package(nlohmann_json_schema_validator REQUIRED)
if(NOT TARGET nlohmann_json_schema_validator)
    add_library(nlohmann_json_schema_validator INTERFACE)
    target_link_libraries(nlohmann_json_schema_validator INTERFACE nlohmann_json_schema_validator::validator)
endif()

# Only the shared libraries the executable uses are recorded as needed
# (Boost's CMake package adds ICU, which nothing here calls).
add_link_options("LINKER:--as-needed")

foreach(library log timer sqlite evse_security ocpp)
    add_subdirectory("${EVEREST_LIB_DIR}/${library}" "${CMAKE_BINARY_DIR}/everest/${library}" EXCLUDE_FROM_ALL)
endforeach()

add_executable(ocpp-interop
    src/main.cpp
    src/cli.cpp
    src/output.cpp
    src/fixture.cpp
    src/registry_v16.cpp
    src/registry_v2.cpp
    src/station.cpp
    src/control.cpp
)

target_compile_options(ocpp-interop PRIVATE -Wall -Wextra -Wno-unused-parameter)

set(ADAPTER_VERSION "dev" CACHE STRING "Version of this adapter")
set(UPSTREAM_VERSION "unknown" CACHE STRING "The pinned everest-core release")
set(UPSTREAM_REVISION "0000000000000000000000000000000000000000" CACHE STRING "Its commit")
target_compile_definitions(ocpp-interop PRIVATE
    ADAPTER_VERSION="${ADAPTER_VERSION}"
    UPSTREAM_VERSION="${UPSTREAM_VERSION}"
    UPSTREAM_REVISION="${UPSTREAM_REVISION}"
)

target_link_libraries(ocpp-interop
    PRIVATE
        everest::ocpp
        everest::log
        Boost::log
        Boost::log_setup
        Threads::Threads
)

install(TARGETS ocpp-interop RUNTIME DESTINATION bin)
