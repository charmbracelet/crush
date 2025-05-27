use std::process::{Command, Child};
use std::env;
use std::fs;
use std::io::Write;
use std::path::PathBuf;
use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::Duration;
use crossterm::{
    cursor,
    event::{self, Event, KeyCode, KeyEvent, KeyModifiers},
    execute,
    style::{Color, Print, ResetColor, SetBackgroundColor, SetForegroundColor},
    terminal::{self, ClearType},
};
use std::io::{stdout, Stdout};

#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;

// Define memfd_create syscall manually for Linux
#[cfg(target_os = "linux")]
const SYS_MEMFD_CREATE: libc::c_long = 319;
#[cfg(target_os = "linux")]
const MFD_CLOEXEC: libc::c_uint = 0x0001;

#[cfg(target_os = "linux")]
unsafe fn memfd_create(name: *const libc::c_char, flags: libc::c_uint) -> libc::c_int {
    libc::syscall(SYS_MEMFD_CREATE, name, flags) as libc::c_int
}

#[derive(Debug, Clone)]
enum PanelType {
    Manual,
    BinaryProcess,
}

#[derive(Debug, Clone)]
struct Panel {
    id: usize,
    title: String,
    content: Vec<String>,
    process: Option<Arc<Mutex<Child>>>,
    is_active: bool,
    cursor_line: usize,
    scroll_offset: usize,
    panel_type: PanelType,
}

impl Panel {
    fn new(id: usize, title: String) -> Self {
        Panel {
            id,
            title,
            content: Vec::new(),
            process: None,
            is_active: false,
            cursor_line: 0,
            scroll_offset: 0,
            panel_type: PanelType::Manual,
        }
    }

    fn new_binary(id: usize, title: String) -> Self {
        Panel {
            id,
            title,
            content: Vec::new(),
            process: None,
            is_active: false,
            cursor_line: 0,
            scroll_offset: 0,
            panel_type: PanelType::BinaryProcess,
        }
    }

    fn add_line(&mut self, line: String) {
        self.content.push(line);
        if self.content.len() > 1000 {
            self.content.remove(0);
        }
    }
}

struct Multiplexer {
    panels: HashMap<usize, Panel>,
    active_panel: usize,
    next_panel_id: usize,
    terminal_size: (u16, u16),
    should_quit: bool,
}

impl Multiplexer {
    fn new() -> Result<Self, Box<dyn std::error::Error>> {
        let (width, height) = terminal::size()?;
        
        let mut mux = Multiplexer {
            panels: HashMap::new(),
            active_panel: 0,
            next_panel_id: 0,
            terminal_size: (width, height),
            should_quit: false,
        };

        // Create the first panel with the embedded Go binary
        mux.create_binary_panel()?;

        Ok(mux)
    }

    fn create_binary_panel(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        let mut panel = Panel::new_binary(self.next_panel_id, "Welcome to OpenCode".to_string());
        panel.is_active = true;
        panel.add_line("Press Enter to start OpenCode, Ctrl+C to switch panels".to_string());
        panel.add_line("When binary is running, it will take full terminal control".to_string());

        self.panels.insert(self.next_panel_id, panel);
        self.active_panel = self.next_panel_id;
        self.next_panel_id += 1;

        Ok(())
    }

    fn execute_go_binary_fullscreen(&self) -> Result<(), Box<dyn std::error::Error>> {
        let go_binary_data = include_bytes!("../joy");
        let args: Vec<String> = env::args().skip(1).collect();
        let args_str: Vec<&str> = args.iter().map(|s| s.as_str()).collect();

        // Disable raw mode to give control back to the binary
        terminal::disable_raw_mode()?;
        
        let mut stdout = stdout();
        execute!(stdout, terminal::Clear(ClearType::All), cursor::MoveTo(0, 0))?;
        stdout.flush()?;

        let result = self.run_binary_with_terminal_control(go_binary_data, &args_str);

        terminal::enable_raw_mode()?;

        match result {
            Ok(_) => Ok(()),
            Err(e) => Err(format!("Failed to execute Go binary: {}", e).into())
        }
    }

    fn run_binary_with_terminal_control(&self, binary_data: &[u8], args: &[&str]) -> Result<(), Box<dyn std::error::Error>> {
        let temp_path = self.create_temp_executable(binary_data)?;
        
        // Execute with inherited stdin/stdout/stderr for full terminal control
        let status = Command::new(&temp_path)
            .args(args)
            .status()?;

        let _ = fs::remove_file(&temp_path);
        if !status.success() {
            return Err("Go binary exited with error".into());
        }

        Ok(())
    }

    fn create_temp_executable(&self, binary_data: &[u8]) -> Result<PathBuf, Box<dyn std::error::Error>> {
        let temp_dir = env::temp_dir();
        let temp_path = temp_dir.join(format!("embedded_go_binary_{}", std::process::id()));
        
        let mut file = fs::File::create(&temp_path)?;
        file.write_all(binary_data)?;
        file.sync_all()?;
        
        #[cfg(unix)]
        {
            let mut perms = fs::metadata(&temp_path)?.permissions();
            perms.set_mode(0o755);
            fs::set_permissions(&temp_path, perms)?;
        }
        
        Ok(temp_path)
    }

    fn create_manual_panel(&mut self, title: String) {
        let mut panel = Panel::new(self.next_panel_id, title);
        panel.add_line("Manual panel created. Type commands or notes here.".to_string());
        panel.add_line("Use Ctrl+C to switch panels, Ctrl+N for new panel, Ctrl+Q to quit.".to_string());
        
        self.panels.insert(self.next_panel_id, panel);
        self.next_panel_id += 1;
    }

    fn switch_to_next_panel(&mut self) {
        if let Some(current) = self.panels.get_mut(&self.active_panel) {
            current.is_active = false;
        }

        let panel_ids: Vec<usize> = self.panels.keys().cloned().collect();
        if let Some(current_idx) = panel_ids.iter().position(|&id| id == self.active_panel) {
            let next_idx = (current_idx + 1) % panel_ids.len();
            self.active_panel = panel_ids[next_idx];
        }

        if let Some(next) = self.panels.get_mut(&self.active_panel) {
            next.is_active = true;
        }
    }

    fn draw(&mut self, stdout: &mut Stdout) -> Result<(), Box<dyn std::error::Error>> {
        execute!(stdout, terminal::Clear(ClearType::All), cursor::MoveTo(0, 0))?;

        // Draw header
        execute!(
            stdout,
            SetBackgroundColor(Color::Blue),
            SetForegroundColor(Color::White),
            Print(format!("OpenCode - {} panels", self.panels.len())),
        )?;

        let header_padding = " ".repeat((self.terminal_size.0 as usize).saturating_sub(30));
        execute!(stdout, Print(header_padding), ResetColor)?;

        execute!(stdout, cursor::MoveTo(0, 1))?;
        for (id, panel) in &self.panels {
            if *id == self.active_panel {
                execute!(
                    stdout,
                    SetBackgroundColor(Color::Green),
                    SetForegroundColor(Color::Black),
                    Print(format!(" {} ", panel.title)),
                    ResetColor,
                    Print(" "),
                )?;
            } else {
                execute!(
                    stdout,
                    SetBackgroundColor(Color::DarkGrey),
                    SetForegroundColor(Color::White),
                    Print(format!(" {} ", panel.title)),
                    ResetColor,
                    Print(" "),
                )?;
            }
        }

        if let Some(active_panel) = self.panels.get(&self.active_panel) {
            let start_row = 3;
            let available_height = (self.terminal_size.1 as usize).saturating_sub(4);
            
            for (i, line) in active_panel.content.iter()
                .skip(active_panel.scroll_offset)
                .take(available_height)
                .enumerate() {
                execute!(
                    stdout,
                    cursor::MoveTo(0, start_row + i as u16),
                    Print(line),
                )?;
            }
        }

        execute!(
            stdout,
            cursor::MoveTo(0, self.terminal_size.1 - 1),
            SetBackgroundColor(Color::DarkGrey),
            SetForegroundColor(Color::White),
        )?;

        let status_text = if let Some(panel) = self.panels.get(&self.active_panel) {
            match panel.panel_type {
                PanelType::BinaryProcess => "Enter: Run Go Binary | Ctrl+C: Switch Panel | Ctrl+N: New Panel | Ctrl+Q: Quit",
                PanelType::Manual => "Ctrl+C: Switch Panel | Ctrl+N: New Panel | Ctrl+Q: Quit",
            }
        } else {
            "Ctrl+C: Switch Panel | Ctrl+N: New Panel | Ctrl+Q: Quit"
        };

        execute!(stdout, Print(status_text))?;

        let status_padding = " ".repeat(
            (self.terminal_size.0 as usize).saturating_sub(status_text.len())
        );
        execute!(stdout, Print(status_padding), ResetColor)?;

        stdout.flush()?;
        Ok(())
    }

    fn handle_input(&mut self, key_event: KeyEvent) -> Result<(), Box<dyn std::error::Error>> {
        match key_event {
            KeyEvent {
                code: KeyCode::Char('q'),
                modifiers: KeyModifiers::CONTROL,
                ..
            } => {
                self.should_quit = true;
            }
            KeyEvent {
                code: KeyCode::Char('c'),
                modifiers: KeyModifiers::CONTROL,
                ..
            } => {
                self.switch_to_next_panel();
            }
            KeyEvent {
                code: KeyCode::Char('n'),
                modifiers: KeyModifiers::CONTROL,
                ..
            } => {
                let panel_name = format!("Panel {}", self.next_panel_id);
                self.create_manual_panel(panel_name);
            }
            KeyEvent {
                code: KeyCode::Enter,
                ..
            } => {
                if let Some(panel) = self.panels.get(&self.active_panel) {
                    match panel.panel_type {
                        PanelType::BinaryProcess => {
                            // Execute Go binary in fullscreen mode
                            if let Err(e) = self.execute_go_binary_fullscreen() {
                                // Add error to panel content
                                if let Some(panel) = self.panels.get_mut(&self.active_panel) {
                                    panel.add_line(format!("Error: {}", e));
                                }
                            } else {
                                if let Some(panel) = self.panels.get_mut(&self.active_panel) {
                                    panel.add_line("Go binary executed successfully!".to_string());
                                }
                            }
                        }
                        PanelType::Manual => {
                            if let Some(panel) = self.panels.get_mut(&self.active_panel) {
                                panel.add_line(">>> Enter pressed".to_string());
                            }
                        }
                    }
                }
            }
            KeyEvent {
                code: KeyCode::Char(c),
                ..
            } => {
                if let Some(panel) = self.panels.get_mut(&self.active_panel) {
                    if matches!(panel.panel_type, PanelType::Manual) {
                        panel.add_line(format!("Input: {}", c));
                    }
                }
            }
            _ => {}
        }
        Ok(())
    }

    fn run(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        terminal::enable_raw_mode()?;
        let mut stdout = stdout();
        execute!(stdout, terminal::Clear(ClearType::All))?;

        loop {
            self.draw(&mut stdout)?;

            if self.should_quit {
                break;
            }

            // Check for input with timeout
            if event::poll(Duration::from_millis(100))? {
                match event::read()? {
                    Event::Key(key_event) => {
                        self.handle_input(key_event)?;
                    }
                    Event::Resize(width, height) => {
                        self.terminal_size = (width, height);
                    }
                    _ => {}
                }
            }
        }

        terminal::disable_raw_mode()?;
        execute!(stdout, terminal::Clear(ClearType::All), cursor::MoveTo(0, 0))?;
        println!("Terminal multiplexer exited.");

        Ok(())
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut mux = Multiplexer::new()?;
    mux.run()?;
    Ok(())
}

// Include the original binary execution functions for compatibility
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
    use std::ffi::CString;
    
    unsafe {
        let name = CString::new("embedded_go_binary")?;
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
    let temp_path = create_temp_executable(binary_data)?;
    
    let result = Command::new(&temp_path)
        .args(args)
        .output();
    
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
    let temp_path = temp_dir.join(format!("embedded_go_binary_{}", std::process::id()));
    
    let mut file = fs::File::create(&temp_path)?;
    file.write_all(binary_data)?;
    file.sync_all()?;
    
    #[cfg(unix)]
    {
        let mut perms = fs::metadata(&temp_path)?.permissions();
        perms.set_mode(0o755);
        fs::set_permissions(&temp_path, perms)?;
    }
    
    Ok(temp_path)
}