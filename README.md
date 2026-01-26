<p align="center">
  <img  width="300" src="https://raw.githubusercontent.com/traiproject/same/refs/heads/main/brand/logo/same_logo.svg" />
</p>
<p align="center">
same
</p>
<p align="center">
Write Once, Build Once, Anywhere
</p>
<p align="center">
  <a href="https://github.com/traiproject/same/releases"><img src="https://img.shields.io/github/release/traiproject/same.svg" alt="Latest Release"></a>
  <a href="https://pkg.go.dev/go.trai.ch/same?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
  <a href="https://github.com/traiproject/same/actions"><img src="https://github.com/traiproject/same/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a>
  <a href="https://codecov.io/gh/traiproject/same"><img src="https://codecov.io/gh/traiproject/same/graph/badge.svg?token=BZ2R4NX0DA" alt="Codecov"></a>
  <a href="https://goreportcard.com/report/go.trai.ch/same"><img src="https://goreportcard.com/badge/go.trai.ch/same" alt="Go Report Card"></a>
</p>

---

`same` is a modern build tool for monorepos designed to make execution hermetic,
deterministic, and fast across any environment.

- **Reliable**: Deterministic builds via [Nix](https://nixos.org/) ensure that
  if it runs on your machine, it runs on any other machine and on CI.
- **Simple**: Minimal configuration with a clean, uncluttered CLI and TUI.
- **Fast**: Snappy execution with aggressive content-addressable caching and
  parallel scheduling.

## Why `same`?

Use `same` if you struggle with:

- **Inconsistent builds:** "It works on my machine" but fails on CI due to
  environmental differences.
- **Complex setups:** Onboarding new developers takes hours installing specific
  compilers and tool versions.
- **Slow CI:** Rebuilding the entire project for a one-line change.

`same` helps by:

- **Locking toolchains:** Uses Nix to ensure every developer and CI runner uses
  the exact same binary versions.
- **Caching everywhere:** Computes input hashes to skip work that has already
  been done.
- **Unifying execution:** One syntax (`same run`) for all tasks, regardless of
  the underlying language (Go, Rust, Node, etc.).

## Stability

`same` is currently in **v0 (Beta)**.

- **Breaking Changes:** The configuration schema (`same.yaml`) and CLI commands
  may change as we refine the design.
- **Production Use:** While reliable for development workflows, please pin
  versions in your CI pipelines.

## Supported Platforms

`same` is built and tested for the following operating systems and
architectures:

| OS        | Architectures                            |
| :-------- | :--------------------------------------- |
| **Linux** | `amd64`, `arm64`                         |
| **macOS** | `amd64` (Intel), `arm64` (Apple Silicon) |

> **Note:** Windows is not supported at this time.

## Install

### Nix

#### Flake Input (Preferred)

Add same to your flake.nix to ensure your team uses the exact same version:

```nix
{
  inputs.same.url = "github:traiproject/same";

  outputs = { self, nixpkgs, same }: {
    devShells.x86_64-linux.default = nixpkgs.legacyPackages.x86_64-linux.mkShell {
      buildInputs = [ same.packages.x86_64-linux.default ];
    };
  };
}
```

#### Nix Profile

Install directly into your profile:

```bash
nix profile install github:traiproject/same
```

### Homebrew

Install via our official tap:

```bash
brew install traiproject/tap/same
```

or

```bash
brew tap traiproject/tap
brew install same
```

### Install from Release

Binaries and packages are available on the
[Releases](https://github.com/traiproject/same/releases) page.

**Debian / Ubuntu (** `.deb` **)**

```bash
curl -LO https://github.com/traiproject/same/releases/download/v0.0.1/same_0.0.1_linux_amd64.deb
sudo dpkg -i same_0.0.1_linux_amd64.deb
```

**Fedora / RHEL (** `.rpm` **)**

```bash
curl -LO https://github.com/traiproject/same/releases/download/v0.0.1/same_0.0.1_linux_amd64.rpm
sudo rpm -i same_0.0.1_linux_amd64.rpm
```

**Alpine Linux (** `.apk` **)**

```bash
curl -LO https://github.com/traiproject/same/releases/download/v0.0.1/same_0.0.1_linux_amd64.apk
apk add same_0.0.1_linux_amd64.apk
```

> **Note:** Binary archives (`tar.gz`) are available for macOS (Intel/Apple
> Silicon) and Linux (amd64/arm64).

## Quickstart

1. Initialize configuration

   Create a same.yaml in the root of your project:

   ```yaml
   version: "1"

   # Define tools needed for your tasks (provisioned via Nix)
   tools:
     go: go@1.25.4
     lint: golangci-lint@2.7.2

   tasks:
     # Define a task
     build:
       input: ["cmd", "internal", "go.mod"]
       cmd: ["go", "build", "-o", "bin/app", "./cmd/main.go"]
       target: ["bin/app"]
       tools: ["go"]

     # Define a dependent task
     lint:
       input: ["**/*.go"]
       cmd: ["golangci-lint", "run"]
       tools: ["lint"]
       dependsOn: ["build"]
   ```
2. **Run a task**

   ```bash
   same run lint
   ```

### Configuration Schema

The `same.yaml` file drives the execution engine.

- `version`: Configuration format version (currently "1").
- `project`: (Optional) Name of the project, only required if using a workspace
  setup.
- `tools`: A map of tool aliases to versions (e.g., `go: go@1.23`). `same` uses
  Nix to provide these hermetically.`tasks`A map of task names to task
  definitions.

**Task Definition:**

- `input`: List of file globs or paths that affect the task output. Used for
  caching hash calculation.
- `cmd`: The command to execute (as a list of strings).
- `target`: List of output files or directories the task produces.
- `tools`: List of tool aliases (defined in the global `tools` section) required
  by this specific task.
- `dependsOn`: List of other tasks that must complete successfully before this
  task runs.
- `environment`: Map of environment variables injected into the task execution.
- `workingDir`: Directory to execute the command in. If a relative path is
  provided, it is relative to the project root (the directory containing
  same.yaml). Defaults to the project root.
