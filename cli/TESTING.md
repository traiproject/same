# Testing Concept: `same` Daemon & CLI

## 1. Core Philosophy

- **Determinism:** Flaky tests are bugs. Use `synctest` for time and `goldie`
  for output comparison.
- **Isolation:** Unit tests must never touch the disk, network, or real
  subprocesses.
- **Strictness:** Every public function requires a test. Legacy code must be
  brought up to standard when touched ("Boy Scout Rule").

---

## 2. Test Layers & Standards

### 2.1 Unit Tests

**Scope:** Individual functions and struct methods. **Location:** `*_test.go`
next to the file being tested. **Private Access:** Use `export_test.go` to test
critical private members (white-box testing).

- **Mandate:** 100% coverage of public methods.
- **Pattern:** Table-Driven Tests are required.
- **Mocks:**
- Use `go.uber.org/mock/gomock` for **all** interface dependencies.
- **Prohibited:** Real I/O, `time.Sleep`, real Nix commands.

- **Example Template:**

```go
func TestService_Resolve(t *testing.T) {
    // 1. Setup Mocks
    ctrl := gomock.NewController(t)
    mockStore := mocks.NewMockStore(ctrl)

    // 2. Define Table
    tests := []struct {
        name    string
        input   string
        setup   func() // Expectation setup
        want    string
        wantErr bool
    }{
        {
            name:  "Happy Path",
            input: "go@1.25",
            setup: func() {
                mockStore.EXPECT().Get(gomock.Any(), "go@1.25").Return("hash123", nil)
            },
            want: "hash123",
        },
    }

    // 3. Execution
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setup()
            svc := NewService(mockStore)
            got, err := svc.Resolve(context.Background(), tt.input)
            
            // Assertions using testify
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

### 2.2 Integration Tests

**Scope:** Interactions with the filesystem, time, and external binaries (Nix).
**Location:** `*_integration_test.go`.

- **External Tools (Nix):**
- Tests requiring Nix **must** run in CI (Nix is available).
- Locally, use `exec.LookPath("nix")`. If missing, call
  `t.Skip("nix binary not found, skipping integration test")`.

- **Time & Concurrency:**
- Use `testing/synctest` (Go 1.25+).
- **Do not** use `time.Sleep` to wait for goroutines. Use `synctest.Run` to
  simulate deterministic clock advancement.

- **Golden Files (Snapshot Testing):**
- Use `github.com/sebdah/goldie/v2` for hashing outputs, generated Nix files,
  and complex text output.
- Run `go test ./... -update` to regenerate golden files.

**Example (Synctest & Goldie):**

```go
func TestNixGenerator_Integration(t *testing.T) {
    // Determine if we can run
    if _, err := exec.LookPath("nix"); err != nil {
        t.Skip("nix binary not found")
    }

    synctest.Run(func() {
        // Setup Filesystem
        fs := createTempFS(t)
        
        // Run Logic (Simulated Time)
        gen := NewGenerator(fs)
        err := gen.Generate()
        require.NoError(t, err)

        // Compare Output with Goldie
        g := goldie.New(t)
        g.Assert(t, "nix_output", fs.ReadFile("flake.nix"))
    })
}
```

### 2.3 End-to-End (E2E) Tests

**Scope:** User-facing CLI commands (`same run`, `same clean`, ...).
**Location:** `e2e/` directory. **Tooling:**
`github.com/rogpeppe/go-internal/testscript`.

- **Structure:**
- Tests defined in `.txtar` files.
- Must verify stdout, stderr, and exit codes.
- Must verify side effects (files created in `.home` or workspace).

- **Environment:**
- `setupE2E` function isolates `HOME`, `PATH`, and `NO_COLOR`.
- No network access allowed (unless explicitly mocked or passed through for
  Nix).
