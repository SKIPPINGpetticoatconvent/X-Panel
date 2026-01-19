use std::process::Command;
use std::thread;
use std::time::{Duration, Instant};

// ANSI color codes
const RESET: &str = "\x1b[0m";
const RED: &str = "\x1b[31m";
const GREEN: &str = "\x1b[32m";
const YELLOW: &str = "\x1b[33m";
const BLUE: &str = "\x1b[34m";
const BOLD: &str = "\x1b[1m";

struct CheckResult {
    name: String,
    success: bool,
    duration: Duration,
    output: String,
    error: Option<String>,
}

struct Check {
    name: &'static str,
    cmd: &'static str,
    args: Vec<&'static str>,
    enabled: bool,
}

fn main() {
    let start = Instant::now();
    println!("{}Starting X-Panel Verification...{}", BOLD, RESET);

    let checks = vec![
        Check {
            name: "Go Format",
            cmd: "gofmt",
            args: vec!["-l", "."],
            enabled: false, // User requested to ignore style
        },
        Check {
            name: "TOML Format",
            cmd: "taplo",
            args: vec!["fmt", "--check"],
            enabled: false, // User requested to ignore style
        },
        Check {
            name: "Go Build",
            cmd: "go",
            args: vec!["build", "./..."],
            enabled: true,
        },
        Check {
            name: "Go Test",
            cmd: "go",
            args: vec!["test", "./..."],
            enabled: true,
        },
        Check {
            name: "GolangCI-Lint",
            cmd: "golangci-lint",
            args: vec!["run", "./..."],
            enabled: true,
        },
        Check {
            name: "NilAway",
            cmd: "nilaway",
            args: vec!["-test=false", "./..."],
            enabled: true,
        },
         Check {
            name: "ShellCheck",
            cmd: "sh",
            // Only show warnings and errors, ignore style/info
            args: vec!["-c", "find . -name '*.sh' | xargs shellcheck --severity=warning"],
            enabled: true,
        },
    ];

    let mut handles = vec![];

    for check in checks {
        if !check.enabled {
            continue;
        }
        let handle = thread::spawn(move || run_check(check));
        handles.push(handle);
    }

    let mut results = vec![];
    for handle in handles {
        results.push(handle.join().unwrap());
    }

    let mut success = true;
    let mut failed_checks = vec![];

    println!("\n---------------------------------------------------");
    for res in &results {
        if res.success {
            println!("{}[PASS]{} {:<15} ({:?})", GREEN, RESET, res.name, res.duration);
        } else {
            println!("{}[FAIL]{} {:<15} ({:?})", RED, RESET, res.name, res.duration);
            success = false;
            failed_checks.push(res);
        }
    }
    println!("---------------------------------------------------");

    if !failed_checks.is_empty() {
        println!("\n{}Failures Details:{}", RED, RESET);
        for fail in failed_checks {
            println!("\n{}--- {} Output ---{}", YELLOW, fail.name, RESET);
            if let Some(err) = &fail.error {
                println!("Execution Error: {}", err);
            }
            if !fail.output.is_empty() {
                println!("{}", fail.output);
            }
            if fail.name == "Go Format" && !fail.output.is_empty() {
                 println!("{}Tip: Run 'gofmt -w .' to fix these files.{}", BLUE, RESET);
            }
        }
    }

    let total_duration = start.elapsed();
    if success {
        println!("\n{}All checks passed in {:?}! 🚀{}", GREEN, total_duration, RESET);
        std::process::exit(0);
    } else {
        println!("\n{}Verification failed in {:?}. Please fix the issues above.{}", RED, total_duration, RESET);
        std::process::exit(1);
    }
}

fn run_check(c: Check) -> CheckResult {
    let start = Instant::now();
    
    let output_res = Command::new(c.cmd)
        .args(&c.args)
        .output();

    let duration = start.elapsed();

    match output_res {
        Ok(output) => {
            let mut is_success = output.status.success();
            let stdout_str = String::from_utf8_lossy(&output.stdout).to_string();
            let stderr_str = String::from_utf8_lossy(&output.stderr).to_string();

            let mut final_output = stdout_str.clone();
            if !stderr_str.is_empty() {
                if !final_output.is_empty() {
                    final_output.push('\n');
                }
                final_output.push_str(&stderr_str);
            }

            // Logic for specific Go Format check
             if c.name == "Go Format" {
                if !stdout_str.trim().is_empty() {
                    is_success = false;
                }
             }

            CheckResult {
                name: c.name.to_string(),
                success: is_success,
                duration,
                output: final_output.trim().to_string(),
                error: None,
            }
        }
        Err(e) => CheckResult {
            name: c.name.to_string(),
            success: false,
            duration,
            output: String::new(),
            error: Some(e.to_string()),
        },
    }
}
