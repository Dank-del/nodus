# CPM

CPM is a Git-native C and C++ source dependency manager written in Go. Its MVP
uses GitHub repositories as package sources, locks every dependency to a commit,
and generates CMake source integration without requiring upstream repackaging.

## Build

```bash
go build -o cpm ./cmd/cpm
```

The CLI requires `git`. CMake packages also require a system CMake installation
(version 3.14 or newer) because CPM configures each package and reads CMake's
File API before generating integration metadata.

## Quick start

```bash
mkdir my-app && cd my-app
cpm init
cpm add github.com/fmtlib/fmt@v11.1.4
cpm install
```

Include the generated file after `project()` in the app's top-level
`CMakeLists.txt`:

```cmake
include("${CMAKE_SOURCE_DIR}/.cpm/generated/dependencies.cmake")
```

CPM preserves upstream CMake targets, so a package such as fmt remains available
as `fmt::fmt`. Repositories that have no `CMakeLists.txt` but expose `include/`
receive an interface target named `cpm::<owner>_<repo>`.

## MVP behavior

- Sources: `github.com/owner/repo`, optional `@tag`, or `#commit`/`#branch`.
- Unversioned dependencies use the highest stable semantic Git tag, or the
  remote default branch when no semantic tag exists; `cpm.lock` always pins the
  resulting commit.
- CPM follows transitive dependencies only when an upstream checkout has its
  own `cpm.toml`. It does not attempt to infer arbitrary dependencies from
  CMake.
- `cpm install` refuses a lockfile that does not match `cpm.toml`.
- `cpm clean` removes only project-local `.cpm` artifacts; shared Git mirrors
  remain in the OS user cache.
