use std::collections::HashMap;
use std::io;
use std::net::IpAddr;
use std::time::{Duration, Instant};

use chrono::NaiveDateTime;
use crossterm::event::{self, Event, KeyCode, KeyModifiers};
use crossterm::terminal::{self, EnterAlternateScreen, LeaveAlternateScreen};
use crossterm::ExecutableCommand;
use ratatui::backend::CrosstermBackend;
use ratatui::layout::{Constraint, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Cell, Row, Table};
use ratatui::Terminal;
use serde::Deserialize;

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Deserialize)]
struct PortSummary {
    port_number: i64,
    #[allow(dead_code)]
    protocol: String,
    #[allow(dead_code)]
    state: String,
}

#[derive(Debug, Clone, Deserialize)]
struct Host {
    #[allow(dead_code)]
    id: i64,
    ip_address: String,
    hostname: Option<String>,
    mac_address: Option<String>,
    first_seen: NaiveDateTime,
    last_seen: NaiveDateTime,
    is_active: bool,
    #[serde(default)]
    ports: Vec<PortSummary>,
}

#[derive(Debug, Clone, Deserialize, Default)]
struct Stats {
    total_hosts: i64,
    active_hosts: i64,
    #[allow(dead_code)]
    total_ports: i64,
    active_ports: i64,
}

// ---------------------------------------------------------------------------
// API client
// ---------------------------------------------------------------------------

async fn fetch_hosts(client: &reqwest::Client, base_url: &str) -> Result<Vec<Host>, String> {
    let url = format!("{}/api/hosts?limit=1000", base_url);
    let resp = client
        .get(&url)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    resp.json().await.map_err(|e| e.to_string())
}

async fn fetch_stats(client: &reqwest::Client, base_url: &str) -> Result<Stats, String> {
    let url = format!("{}/api/stats", base_url);
    let resp = client
        .get(&url)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    resp.json().await.map_err(|e| e.to_string())
}

// ---------------------------------------------------------------------------
// Reverse DNS cache
// ---------------------------------------------------------------------------

/// Cached reverse DNS results. None = lookup attempted but failed.
type DnsCache = HashMap<String, Option<String>>;

/// Reverse-lookup a single IP (blocking, meant for spawn_blocking).
fn reverse_lookup(ip: String) -> (String, Option<String>) {
    let addr: IpAddr = match ip.parse() {
        Ok(a) => a,
        Err(_) => return (ip, None),
    };
    // getnameinfo via the standard library
    let result = dns_lookup::lookup_addr(&addr).ok();
    // Filter out results that are just the IP echoed back
    let result = result.filter(|name| name != &ip);
    (ip, result)
}

/// Resolve hostnames for IPs that aren't in the cache yet.
/// Runs lookups concurrently on the blocking threadpool.
async fn resolve_missing(hosts: &[Host], cache: &mut DnsCache) {
    let missing: Vec<String> = hosts
        .iter()
        .filter(|h| h.hostname.is_none() && !cache.contains_key(&h.ip_address))
        .map(|h| h.ip_address.clone())
        .collect();

    if missing.is_empty() {
        return;
    }

    let futs: Vec<_> = missing
        .into_iter()
        .map(|ip| tokio::task::spawn_blocking(move || reverse_lookup(ip)))
        .collect();

    let results = futures::future::join_all(futs).await;
    for result in results {
        if let Ok((ip, name)) = result {
            cache.insert(ip, name);
        }
    }
}

// ---------------------------------------------------------------------------
// App state
// ---------------------------------------------------------------------------

struct App {
    hosts: Vec<Host>,
    stats: Stats,
    scroll_offset: usize,
    last_updated: Option<Instant>,
    error: Option<String>,
    base_url: String,
    dns_cache: DnsCache,
}

impl App {
    fn new(base_url: String) -> Self {
        Self {
            hosts: Vec::new(),
            stats: Stats::default(),
            scroll_offset: 0,
            last_updated: None,
            error: None,
            base_url,
            dns_cache: HashMap::new(),
        }
    }

    fn sorted_hosts(&self) -> Vec<&Host> {
        let mut hosts: Vec<&Host> = self.hosts.iter().collect();
        hosts.sort_by(|a, b| {
            b.is_active
                .cmp(&a.is_active)
                .then_with(|| cmp_ip(&a.ip_address, &b.ip_address))
        });
        hosts
    }

    fn hostname_for<'a>(&'a self, host: &'a Host) -> Option<&'a str> {
        if let Some(ref name) = host.hostname {
            return Some(name.as_str());
        }
        self.dns_cache
            .get(&host.ip_address)
            .and_then(|opt| opt.as_deref())
    }

    fn scroll_up(&mut self) {
        self.scroll_offset = self.scroll_offset.saturating_sub(1);
    }

    fn scroll_down(&mut self) {
        let max = self.hosts.len().saturating_sub(1);
        if self.scroll_offset < max {
            self.scroll_offset += 1;
        }
    }

    fn scroll_home(&mut self) {
        self.scroll_offset = 0;
    }

    fn scroll_end(&mut self) {
        self.scroll_offset = self.hosts.len().saturating_sub(1);
    }
}

/// Compare IP addresses numerically by parsing octets.
fn cmp_ip(a: &str, b: &str) -> std::cmp::Ordering {
    let parse = |s: &str| -> Vec<u32> {
        s.split('.').filter_map(|p| p.parse().ok()).collect()
    };
    parse(a).cmp(&parse(b))
}

fn fmt_date(dt: &NaiveDateTime) -> String {
    dt.format("%Y-%m-%d").to_string()
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

fn draw(frame: &mut ratatui::Frame, app: &App) {
    let area = frame.area();

    // Layout: top bar (1), table (rest), bottom bar (1)
    let chunks = Layout::vertical([
        Constraint::Length(1),
        Constraint::Min(3),
        Constraint::Length(1),
    ])
    .split(area);

    draw_top_bar(frame, chunks[0], app);
    draw_table(frame, chunks[1], app);
    draw_bottom_bar(frame, chunks[2], app);
}

fn draw_top_bar(frame: &mut ratatui::Frame, area: Rect, app: &App) {
    let stats_text = format!(
        "{} hosts ({} active)  {} open ports",
        app.stats.total_hosts, app.stats.active_hosts, app.stats.active_ports
    );

    let line = Line::from(vec![
        Span::styled(
            " Network Dashboard ",
            Style::default()
                .fg(Color::Cyan)
                .add_modifier(Modifier::BOLD),
        ),
        Span::raw("── "),
        Span::styled(stats_text, Style::default().fg(Color::White)),
    ]);

    frame.render_widget(line, area);
}

fn draw_table(frame: &mut ratatui::Frame, area: Rect, app: &App) {
    let header = Row::new(vec![
        Cell::from("Status"),
        Cell::from("IP Address"),
        Cell::from("Hostname"),
        Cell::from("MAC Address"),
        Cell::from("Ports"),
        Cell::from("First Seen"),
        Cell::from("Last Seen"),
    ])
    .style(
        Style::default()
            .fg(Color::Yellow)
            .add_modifier(Modifier::BOLD),
    );

    let hosts = app.sorted_hosts();
    let rows: Vec<Row> = hosts
        .iter()
        .skip(app.scroll_offset)
        .map(|h| {
            let (status_icon, style) = if h.is_active {
                ("● UP", Style::default().fg(Color::Green))
            } else {
                ("○ DOWN", Style::default().fg(Color::DarkGray))
            };

            let ports_str = if h.ports.is_empty() {
                "—".to_string()
            } else {
                let mut nums: Vec<i64> = h.ports.iter().map(|p| p.port_number).collect();
                nums.sort();
                nums.iter()
                    .map(|n| n.to_string())
                    .collect::<Vec<_>>()
                    .join(", ")
            };

            let hostname = app.hostname_for(h).unwrap_or("—");

            Row::new(vec![
                Cell::from(status_icon),
                Cell::from(h.ip_address.as_str()),
                Cell::from(hostname.to_owned()),
                Cell::from(
                    h.mac_address
                        .as_deref()
                        .unwrap_or("—")
                        .to_owned(),
                ),
                Cell::from(ports_str),
                Cell::from(fmt_date(&h.first_seen)),
                Cell::from(fmt_date(&h.last_seen)),
            ])
            .style(style)
        })
        .collect();

    let widths = [
        Constraint::Length(8),
        Constraint::Length(16),
        Constraint::Length(20),
        Constraint::Length(19),
        Constraint::Fill(1),
        Constraint::Length(12),
        Constraint::Length(12),
    ];

    let table = Table::new(rows, widths)
        .header(header)
        .block(Block::default().borders(Borders::NONE));

    frame.render_widget(table, area);
}

fn draw_bottom_bar(frame: &mut ratatui::Frame, area: Rect, app: &App) {
    let left = match (&app.error, app.last_updated) {
        (Some(err), _) => Span::styled(
            format!(" Connection error: {}", truncate(err, 60)),
            Style::default().fg(Color::Red),
        ),
        (None, Some(instant)) => {
            let ago = instant.elapsed().as_secs();
            Span::styled(
                format!(" Updated {}s ago", ago),
                Style::default().fg(Color::DarkGray),
            )
        }
        (None, None) => Span::styled(" Connecting...", Style::default().fg(Color::Yellow)),
    };

    let right = Span::styled(
        "q to quit ",
        Style::default().fg(Color::DarkGray),
    );

    // Pad middle to push right text to the edge
    let left_len = left.width();
    let right_len = right.width();
    let padding = (area.width as usize).saturating_sub(left_len + right_len);

    let line = Line::from(vec![left, Span::raw(" ".repeat(padding)), right]);
    frame.render_widget(line, area);
}

fn truncate(s: &str, max: usize) -> &str {
    if s.len() <= max {
        s
    } else {
        &s[..max]
    }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

#[tokio::main]
async fn main() -> io::Result<()> {
    // Parse --url flag or NETDASH_URL env
    let base_url = parse_base_url();

    let mut app = App::new(base_url);
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(10))
        .build()
        .expect("failed to build HTTP client");

    // Terminal setup
    terminal::enable_raw_mode()?;
    io::stdout().execute(EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(io::stdout());
    let mut terminal = Terminal::new(backend)?;

    let poll_interval = Duration::from_secs(5);
    let mut last_poll = Instant::now() - poll_interval; // trigger immediate fetch

    loop {
        // Poll API data
        if last_poll.elapsed() >= poll_interval {
            last_poll = Instant::now();
            let (hosts_res, stats_res) = tokio::join!(
                fetch_hosts(&client, &app.base_url),
                fetch_stats(&client, &app.base_url),
            );

            match hosts_res {
                Ok(hosts) => {
                    app.hosts = hosts;
                    app.error = None;
                    app.last_updated = Some(Instant::now());
                    // Resolve missing hostnames via reverse DNS
                    resolve_missing(&app.hosts, &mut app.dns_cache).await;
                }
                Err(e) => {
                    app.error = Some(e);
                }
            }

            if let Ok(stats) = stats_res {
                app.stats = stats;
            }
        }

        // Draw
        terminal.draw(|frame| draw(frame, &app))?;

        // Handle input (100ms timeout so we redraw regularly)
        if event::poll(Duration::from_millis(100))? {
            if let Event::Key(key) = event::read()? {
                match key.code {
                    KeyCode::Char('q') => break,
                    KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => break,
                    KeyCode::Up | KeyCode::Char('k') => app.scroll_up(),
                    KeyCode::Down | KeyCode::Char('j') => app.scroll_down(),
                    KeyCode::Home => app.scroll_home(),
                    KeyCode::End => app.scroll_end(),
                    _ => {}
                }
            }
        }
    }

    // Cleanup
    terminal::disable_raw_mode()?;
    io::stdout().execute(LeaveAlternateScreen)?;
    Ok(())
}

fn parse_base_url() -> String {
    let args: Vec<String> = std::env::args().collect();

    // Check for --url <value>
    for i in 0..args.len() {
        if args[i] == "--url" {
            if let Some(url) = args.get(i + 1) {
                return url.trim_end_matches('/').to_string();
            }
        }
    }

    // Fall back to env var
    if let Ok(url) = std::env::var("NETDASH_URL") {
        return url.trim_end_matches('/').to_string();
    }

    "http://localhost:3000".to_string()
}
