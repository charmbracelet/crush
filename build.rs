use std::env;
use std::process::Command;

fn main() {
    let root_dir = env::var("CARGO_MANIFEST_DIR").unwrap();

    // Compile Go code to a binary
    let output = Command::new("go")
        .args(&["build", "-o"])
        .arg(&format!("{}/opencode", root_dir))
        .output()
        .expect("Failed to compile opencode");

    if !output.status.success() {
        panic!(
            "Go compilation failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
    }

    // Tell cargo to rerun build script if Go files change
    println!("cargo:rerun-if-changed={}/main.go", root_dir);
    println!("cargo:rerun-if-changed={}/go.mod", root_dir);
    println!("cargo:rerun-if-changed={}", format!("{}/cmd", root_dir));
    println!(
        "cargo:rerun-if-changed={}",
        format!("{}/internal", root_dir)
    );
}
