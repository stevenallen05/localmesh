// Generates src/proto/hello.rs from ../proto/hello.proto on every build.
// Generated file is committed (see DESIGN_DECISIONS.md "Proto codegen") so the
// wire shape is visible in diffs and the Docker build can run without protoc-
// regenerating into a hidden OUT_DIR. Output is deterministic; no-op if the
// proto hasn't changed.
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let out = "src/proto";
    std::fs::create_dir_all(out)?;
    tonic_build::configure()
        .build_server(true)
        .build_client(false)
        .out_dir(out)
        .compile(&["../proto/hello.proto"], &["../proto"])?;
    println!("cargo:rerun-if-changed=../proto/hello.proto");
    Ok(())
}
