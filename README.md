# CPM

> **Pre-release — v0.0.1-alpha.** CPM is under active development. Its CLI,
> manifest, and lockfile formats may change before the first stable release.

CPM is a Git-native C and C++ dependency manager and beginner-friendly CMake
frontend written in Go. It uses GitHub repositories as package sources, locks
every resolved dependency to a commit, and can generate a complete CMake project
without requiring upstream libraries to be repackaged.

The goal is a familiar, lightweight workflow for the common case: use a Git
repository directly, pin it immutably, and consume its existing CMake targets
from an existing CMake project.

## What CPM does

```mermaid
flowchart LR
    A["cpm add github.com/owner/repo@tag"] --> B[Resolve Git ref]
    B --> C[Pin full commit in cpm.lock]
    C --> D[Materialize source in .cpm/packages]
    D --> E[Configure CMake and inspect targets]
    E --> F[Generate dependencies.cmake]
    F --> G[Your existing CMake build]
```

CPM currently supports:

- GitHub shorthand, HTTPS, and SSH repository sources.
- Semantic Git tag selection, explicit tags, commits, and branches.
- Commit-pinned `cpm.lock` files and stale-lock protection.
- Transitive dependencies declared by an upstream `cpm.toml`.
- Native CMake projects through `add_subdirectory()`.
- Header-only repositories with a conventional `include/` directory.
- Project-local source/build output with shared Git mirrors in the OS cache.

It does **not** yet support other Git hosts, SemVer ranges, package adapters,
binary packages, arbitrary CMake dependency inference, or cross-compilation.
It is not a replacement for CMake, Conan, or vcpkg.

## Requirements

- Go 1.26 or newer to build CPM from source.
- Git available on `PATH`; CPM intentionally uses your system Git credentials,
  SSH configuration, and credential helpers.
- CMake 3.14 or newer to add or install a CMake dependency. CPM configures the
  package and reads the CMake File API before creating integration metadata.

## Install the CLI

Build from a checkout:

```bash
git clone https://github.com/Dank-del/cpm.git
cd cpm
go build -o cpm ./cmd/cpm
./cpm version
```

Or install it into your Go bin directory:

```bash
go install github.com/Dank-del/cpm/cmd/cpm@latest
export PATH="$(go env GOPATH)/bin:$PATH"
cpm version
```

## Quick start

### Start a new project

```bash
cpm new hello
cd hello
cpm build
cpm run
cpm test
```

`cpm new` creates a C++20 executable, CTest smoke test, CMake build file, and
project-local CPM metadata. Use `--type library` for a static library template
or `--git` to initialize a Git repository.

For the common workflow, CPM owns the generated `CMakeLists.txt`; edit
`cpm.toml` through CPM commands instead:

```bash
cpm source add extras/feature.cpp
cpm add github.com/fmtlib/fmt@v11.1.4
cpm link fmt::fmt
cpm build --release
cpm run -- --verbose
```

CPM automatically discovers C/C++ files under `src/`, while `cpm source add`
tracks project-relative files outside `src/` explicitly. `cpm link` accepts
only targets discovered in the resolved CPM lockfile, preventing misspelled or
unknown links.

Long-running dependency operations show colored terminal progress for Git
resolution, source materialization, and CMake inspection. CPM keeps redirected
output clean for CI and scripts.

### Add CPM to an existing project

```bash
mkdir my-app && cd my-app
cpm init
cpm add github.com/fmtlib/fmt@v11.1.4
cpm add github.com/nlohmann/json@v3.11.3
cpm install
```

`cpm init` creates a dependency manifest without taking ownership of an
existing `CMakeLists.txt`:

```toml
[project]
name = "my-app"
version = "0.0.1-alpha"

[dependencies]
fmt = "github.com/fmtlib/fmt@v11.1.4"
json = "github.com/nlohmann/json@v3.11.3"
```

After `cpm install`, include the generated file after `project()` in your
top-level `CMakeLists.txt`:

```cmake
cmake_minimum_required(VERSION 3.14)
project(my_app LANGUAGES CXX)

include("${CMAKE_SOURCE_DIR}/.cpm/generated/dependencies.cmake")

add_executable(my_app src/main.cpp)
target_link_libraries(my_app PRIVATE fmt::fmt nlohmann_json::nlohmann_json)
```

Then build your project normally:

```bash
cmake -S . -B build
cmake --build build
```

## How the files fit together

```mermaid
flowchart TD
    M["cpm.toml\nrequested direct dependencies"] --> R[CPM resolver]
    R --> L["cpm.lock\ncanonical source + immutable commit graph"]
    L --> P[".cpm/packages\nproject-local detached checkouts"]
    L --> G[".cpm/generated/dependencies.cmake"]
    G --> C["CMakeLists.txt"]
    C --> B["cmake --build"]
    Cache["User cache: cached Git mirrors"] -. speeds up fetches .-> R
```

- `cpm.toml` is the human-edited manifest.
- `cpm.lock` is the reproducible, generated source graph. Commit this file.
- `.cpm/` is generated project-local state. Do not commit it.
- The shared cache is an optimization only; it does not affect locked commits.

## Command reference

| Command | Purpose |
| --- | --- |
| `cpm version` | Print the CPM version. |
| `cpm about` | Show CPM/project status plus detected Git, CMake, and C/C++ toolchains. It does not configure or compile a project. |
| `cpm new NAME` | Scaffold a CPM-managed C++20 executable project. Use `--type library`, `--cpp-standard N`, or `--git` as needed. |
| `cpm init [--name NAME]` | Create a project manifest. |
| `cpm add [--name ALIAS] SOURCE` | Add, resolve, validate, and lock a dependency. |
| `cpm remove ALIAS` | Remove a direct dependency and rebuild the lock graph. |
| `cpm install` | Materialize exactly the graph in `cpm.lock`. |
| `cpm update [ALIAS...]` | Re-resolve the current manifest. |
| `cpm source add/remove PATH...` | Add or remove explicit C/C++ source entries in a managed project. |
| `cpm link/unlink TARGET...` | Add or remove resolved CPM target links in a managed project. |
| `cpm build [--release]` | Configure and build a managed project. |
| `cpm run [--release] -- [ARGS...]` | Build and run a managed executable. |
| `cpm test [--release]` | Build a managed project with CTest enabled and run its tests. |
| `cpm list` | Show locked packages. |
| `cpm tree` | Show the locked dependency tree. |
| `cpm inspect SOURCE` | Resolve and inspect a remote package and its CMake targets. |
| `cpm clean` | Remove this project’s `.cpm/` artifacts only. |

### Source forms

```bash
cpm add github.com/fmtlib/fmt                 # highest stable SemVer tag
cpm add github.com/fmtlib/fmt@v11.1.4          # exact Git tag
cpm add github.com/owner/project#abc123        # exact commit
cpm add github.com/owner/project#main          # branch, locked to its current commit
cpm add --name fmtlib git@github.com:fmtlib/fmt.git@v11.1.4
```

An unversioned repository resolves to its highest stable semantic version tag.
If it has no semantic tags, CPM resolves the remote default branch and pins the
resulting commit in the lockfile. `cpm install` will never silently move that
commit; use `cpm update` to resolve again.

## CMake integration notes

CPM preserves the targets created by upstream CMake projects. Use the target
names documented by that project, such as `fmt::fmt`. For a repository without
`CMakeLists.txt` that exposes `include/`, CPM generates the interface target
`cpm::<owner>_<repo>`.

Every CMake dependency is configured in an isolated `.cpm/build/` directory.
If configuration fails, CPM stops and preserves `cpm-configure.log` so the
upstream failure can be diagnosed without losing the checkout.

### CMake options

Some upstream projects expose optional features or require platform-specific
CMake cache settings. Pass them generically—without CPM carrying library-
specific behavior—when adding or inspecting a dependency:

```bash
cpm add --cmake-option FOO_BUILD_TESTS=OFF github.com/example/foo
cpm inspect --cmake-option FOO_USE_SYSTEM_DEPENDENCY=ON github.com/example/foo
```

Options added with `cpm add` are stored by dependency alias in `cpm.toml` and
recorded in `cpm.lock`, so `install`, `update`, and managed-project builds use
the same CMake configuration:

```toml
[cmake_options]
foo = ["FOO_BUILD_TESTS=OFF"]
```

## HTTP client end-to-end example

After installing CMake, this is a compact real-world test of dependency
resolution, target linking, generation, compilation, and network execution:

```bash
cpm new http-demo
cd http-demo
cpm add github.com/libcpr/cpr@1.11.0
cpm link cpr::cpr
```

Replace `src/main.cpp` with:

```cpp
#include <cpr/cpr.h>
#include <iostream>

int main() {
    const auto response = cpr::Get(cpr::Url{"https://httpbin.org/get"});
    std::cout << "HTTP status: " << response.status_code << '\n';
    return response.status_code == 200 ? 0 : 1;
}
```

Then run `cpm run`. The public `httpbin.org/get` endpoint should return HTTP
200. This example requires network access and CMake; CPM itself does not make
network calls at application runtime.

## Development status

`v0.0.1-alpha` is an experimental MVP version line and should not be considered
stable for production workflows. Pin CPM itself to an exact commit or release
tag if you automate it. Until a proper release is announced, breaking changes
are expected.

## License

CPM is distributed under the [MIT License](LICENSE).

---

Developed by Sayan Biswas · [@Dank-del](https://github.com/Dank-del)
