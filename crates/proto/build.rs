use std::env;
use std::path::PathBuf;
use std::process::Command;

fn main() {
    let out_dir = env::var("OUT_DIR").unwrap();
    let manifest_dir = env::var("CARGO_MANIFEST_DIR").unwrap();

    let workspace_root = find_workspace_root(&manifest_dir)
        .expect("Could not find workspace root containing 'buf.yaml'");

    // 3. Use absolute paths for rebuild triggers (safer than relative)
    println!(
        "cargo:rerun-if-changed={}",
        workspace_root.join("proto").display()
    );
    println!(
        "cargo:rerun-if-changed={}",
        workspace_root.join("buf.yaml").display()
    );
    println!(
        "cargo:rerun-if-changed={}",
        workspace_root.join("buf.gen.yaml").display()
    );
    println!(
        "cargo:rerun-if-changed={}",
        workspace_root.join("buf.lock").display()
    );

    let status = Command::new("buf")
        .current_dir(&workspace_root)
        .args(["generate", "--output", &out_dir])
        .status()
        .expect("Failed to run 'buf'. Ensure it is installed and in your PATH.");

    if !status.success() {
        panic!("'buf generate' failed. Check the logs above for details.");
    }
}

/// Start at the crate directory and go up until we find `buf.yaml`
fn find_workspace_root(start_path: &str) -> Option<PathBuf> {
    let mut path = PathBuf::from(start_path);
    loop {
        if path.join("buf.yaml").exists() {
            return Some(path);
        }
        if !path.pop() {
            return None;
        }
    }
}
