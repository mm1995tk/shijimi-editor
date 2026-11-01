// Windows の release で黒い窓を出さない（対応 OS ではないが、害は無い）。
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    tauri::Builder::default()
        .run(tauri::generate_context!())
        .expect("failed to run the shijimi-editor GUI");
}
