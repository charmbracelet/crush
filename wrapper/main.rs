use std::process::Command;
use std::env;
use std::fs;
use std::io::Write;
use std::path::PathBuf;

#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;

// Define memfd_create syscall manually for Linux
#[cfg(target_os = "linux")]
const SYS_MEMFD_CREATE: libc::c_long = 319; // x86_64 Linux
#[cfg(target_os = "linux")]
const MFD_CLOEXEC: libc::c_uint = 0x0001;

#[cfg(target_os = "linux")]
unsafe fn memfd_create(name: *const libc::c_char, flags: libc::c_uint) -> libc::c_int {
    libc::syscall(SYS_MEMFD_CREATE, name, flags) as libc::c_int
}

fn main() {
    let go_binary_data = include_bytes!("../joy");
    
    let args: Vec<String> = env::args().skip(1).collect();
    let args_str: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
        
    match run_from_memory_or_temp(go_binary_data, &args_str) {
        Ok(output) => {
            if !output.is_empty() {
                print!("{}", output);
            }
            println!("Go program executed successfully!");
        }
        Err(e) => {
            eprintln!("Failed to execute Go binary: {}", e);
            std::process::exit(1);
        }
    }
}

pub fn run_from_memory_or_temp(binary_data: &[u8], args: &[&str]) -> Result<String, Box<dyn std::error::Error>> {
    #[cfg(target_os = "linux")]
    {
        match run_from_memory_linux(binary_data, args) {
            Ok(result) => return Ok(result),
            Err(e) => {
                eprintln!("Linux memory execution failed ({}), falling back to temp file", e);
            }
        }
    }
    
    #[cfg(target_os = "macos")]
    {
        match run_from_memory_macos(binary_data, args) {
            Ok(result) => return Ok(result),
            Err(e) => {
                eprintln!("macOS memory execution failed ({}), falling back to temp file", e);
            }
        }
    }
    
    run_from_temp_file(binary_data, args)
}

#[cfg(target_os = "linux")]
pub fn run_from_memory_linux(binary_data: &[u8], args: &[&str]) -> Result<String, Box<dyn std::error::Error>> {
    unsafe {
        let name = CString::new("opencode")?;
        let fd = memfd_create(name.as_ptr(), MFD_CLOEXEC);
        if fd == -1 {
            return Err("Failed to create memfd".into());
        }
        
        let written = libc::write(fd, binary_data.as_ptr() as *const libc::c_void, binary_data.len());
        if written != binary_data.len() as isize {
            libc::close(fd);
            return Err("Failed to write binary data to memfd".into());
        }
        
        let proc_path = format!("/proc/self/fd/{}", fd);
        
        let output = Command::new(&proc_path)
            .args(args)
            .output()?;
        
        libc::close(fd);
        
        if output.status.success() {
            Ok(String::from_utf8(output.stdout)?)
        } else {
            Err(format!("Go binary failed: {}", String::from_utf8_lossy(&output.stderr)).into())
        }
    }
}

#[cfg(target_os = "macos")]
pub fn run_from_memory_macos(binary_data: &[u8], args: &[&str]) -> Result<String, Box<dyn std::error::Error>> {
    use std::ffi::CString;
    
    unsafe {
        let template = CString::new("/tmp/opencode")?;
        let mut template_bytes = template.into_bytes_with_nul();
        
        let fd = libc::mkstemp(template_bytes.as_mut_ptr() as *mut libc::c_char);
        if fd == -1 {
            return Err(format!("Failed to create temporary file: errno {}", 
                             *libc::__error()).into());
        }
        
        let temp_path_cstr = CString::from_vec_with_nul(template_bytes)?;
        let temp_path = temp_path_cstr.to_str()?.to_string();
        
        let mut written = 0;
        while written < binary_data.len() {
            let result = libc::write(
                fd,
                binary_data[written..].as_ptr() as *const libc::c_void,
                binary_data.len() - written
            );
            
            if result == -1 {
                let err = *libc::__error();
                libc::close(fd);
                libc::unlink(temp_path_cstr.as_ptr());
                return Err(format!("Failed to write to temporary file: errno {}", err).into());
            }
            
            written += result as usize;
        }
        
        if libc::fchmod(fd, 0o755) == -1 {
            let err = *libc::__error();
            libc::close(fd);
            libc::unlink(temp_path_cstr.as_ptr());
            return Err(format!("Failed to make file executable: errno {}", err).into());
        }
        
        libc::close(fd);
        
        let output = Command::new(&temp_path)
            .args(args)
            .output();
        
        libc::unlink(temp_path_cstr.as_ptr());        
        match output {
            Ok(output) => {
                if output.status.success() {
                    Ok(String::from_utf8(output.stdout)?)
                } else {
                    Err(format!("Go binary failed: {}", String::from_utf8_lossy(&output.stderr)).into())
                }
            }
            Err(e) => Err(e.into())
        }
    }
}

pub fn run_from_temp_file(binary_data: &[u8], args: &[&str]) -> Result<String, Box<dyn std::error::Error>> {
    // Create a temporary executable file
    let temp_path = create_temp_executable(binary_data)?;
    
    // Execute the temporary binary
    let result = Command::new(&temp_path)
        .args(args)
        .output();
    
    // Clean up the temporary file
    let _ = fs::remove_file(&temp_path);
    
    match result {
        Ok(output) => {
            if output.status.success() {
                Ok(String::from_utf8(output.stdout)?)
            } else {
                Err(format!("Go binary failed: {}", String::from_utf8_lossy(&output.stderr)).into())
            }
        }
        Err(e) => Err(e.into())
    }
}

fn create_temp_executable(binary_data: &[u8]) -> Result<PathBuf, Box<dyn std::error::Error>> {
    let temp_dir = env::temp_dir();
    let temp_path = temp_dir.join(format!("opencode_{}", std::process::id()));
    
    let mut file = fs::File::create(&temp_path)?;
    file.write_all(binary_data)?;
    file.sync_all()?;
    
    // make the file executable (Unix/macOS)
    #[cfg(unix)]
    {
        let mut perms = fs::metadata(&temp_path)?.permissions();
        perms.set_mode(0o755); // basically rwxr-xr-x
        fs::set_permissions(&temp_path, perms)?;
    }
    
    Ok(temp_path)
}

// TODO: Idk if i need this
#[allow(dead_code)]
pub fn run_with_streaming_output(binary_data: &[u8], args: &[&str]) -> Result<(), Box<dyn std::error::Error>> {
    let temp_path = create_temp_executable(binary_data)?;
    
    let result = Command::new(&temp_path)
        .args(args)
        .stdout(std::process::Stdio::inherit())
        .stderr(std::process::Stdio::inherit())
        .status();
    
    let _ = fs::remove_file(&temp_path);
    
    match result {
        Ok(status) => {
            if !status.success() {
                return Err(format!("Go binary failed with exit code: {}", 
                                 status.code().unwrap_or(-1)).into());
            }
            Ok(())
        }
        Err(e) => Err(e.into())
    }
}