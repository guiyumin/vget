use base64::{engine::general_purpose::STANDARD, Engine};
use headless_chrome::{types::PrintToPdfOptions, Browser, LaunchOptions};
use pulldown_cmark::{CodeBlockKind, Event, HeadingLevel, Options, Parser, Tag, TagEnd};
use std::fs;
use std::path::Path;
use syntect::highlighting::ThemeSet;
use syntect::html::highlighted_html_for_string;
use syntect::parsing::SyntaxSet;

// Embed fonts at compile time
static INTER_REGULAR: &[u8] = include_bytes!("../resources/fonts/Inter-Regular.woff2");
static INTER_MEDIUM: &[u8] = include_bytes!("../resources/fonts/Inter-Medium.woff2");
static INTER_SEMIBOLD: &[u8] = include_bytes!("../resources/fonts/Inter-SemiBold.woff2");
static INTER_BOLD: &[u8] = include_bytes!("../resources/fonts/Inter-Bold.woff2");
static INTER_EXTRABOLD: &[u8] = include_bytes!("../resources/fonts/Inter-ExtraBold.woff2");
static JETBRAINS_MONO_REGULAR: &[u8] = include_bytes!("../resources/fonts/JetBrainsMono-Regular.woff2");
static JETBRAINS_MONO_MEDIUM: &[u8] = include_bytes!("../resources/fonts/JetBrainsMono-Medium.woff2");

/// Convert markdown file to PDF
pub fn convert_md_to_pdf(
    input_path: &str,
    output_path: &str,
    theme: &str,
    page_size: &str,
) -> Result<(), String> {
    // Read markdown file
    let markdown = fs::read_to_string(input_path)
        .map_err(|e| format!("Failed to read markdown file: {}", e))?;

    // Parse markdown to HTML with syntax highlighting
    let html_content = markdown_to_html(&markdown, theme);

    // Generate full HTML with styling
    let full_html = generate_styled_html(&html_content, theme);

    // Convert HTML to PDF using headless Chrome
    html_to_pdf(&full_html, output_path, page_size)?;

    Ok(())
}

/// Parse markdown to HTML using pulldown-cmark with syntax highlighting
fn markdown_to_html(markdown: &str, theme: &str) -> String {
    let mut options = Options::empty();
    options.insert(Options::ENABLE_TABLES);
    options.insert(Options::ENABLE_FOOTNOTES);
    options.insert(Options::ENABLE_STRIKETHROUGH);
    options.insert(Options::ENABLE_TASKLISTS);
    options.insert(Options::ENABLE_HEADING_ATTRIBUTES);

    let parser = Parser::new_ext(markdown, options);

    // Load syntax highlighting
    let ss = SyntaxSet::load_defaults_newlines();
    let ts = ThemeSet::load_defaults();
    let syntax_theme = if theme == "dark" {
        &ts.themes["base16-ocean.dark"]
    } else {
        &ts.themes["InspiredGitHub"]
    };

    let mut html_output = String::new();
    let mut in_code_block = false;
    let mut in_table_head = false;
    let mut code_lang = String::new();
    let mut code_content = String::new();

    for event in parser {
        match event {
            Event::Start(Tag::CodeBlock(kind)) => {
                in_code_block = true;
                code_content.clear();
                code_lang = match kind {
                    CodeBlockKind::Fenced(lang) => lang.to_string(),
                    CodeBlockKind::Indented => String::new(),
                };
            }
            Event::End(TagEnd::CodeBlock) => {
                in_code_block = false;
                // Try to find syntax for the language
                let syntax = if !code_lang.is_empty() {
                    ss.find_syntax_by_token(&code_lang)
                } else {
                    None
                }
                .unwrap_or_else(|| ss.find_syntax_plain_text());

                // Generate highlighted HTML
                match highlighted_html_for_string(&code_content, &ss, syntax, syntax_theme) {
                    Ok(highlighted) => {
                        html_output.push_str(&highlighted);
                    }
                    Err(_) => {
                        // Fallback to plain code block
                        html_output.push_str("<pre><code>");
                        html_output.push_str(&html_escape(&code_content));
                        html_output.push_str("</code></pre>\n");
                    }
                }
            }
            Event::Text(text) if in_code_block => {
                code_content.push_str(&text);
            }
            Event::Start(Tag::Table(alignments)) => {
                html_output.push_str("<table>\n");
                // Store alignments for later use (simplified - we just open the table)
                let _ = alignments;
            }
            Event::End(TagEnd::Table) => {
                html_output.push_str("</tbody>\n</table>\n");
            }
            Event::Start(Tag::TableHead) => {
                in_table_head = true;
                html_output.push_str("<thead>\n");
            }
            Event::End(TagEnd::TableHead) => {
                in_table_head = false;
                html_output.push_str("</thead>\n<tbody>\n");
            }
            Event::Start(Tag::TableRow) => {
                html_output.push_str("<tr>\n");
            }
            Event::End(TagEnd::TableRow) => {
                html_output.push_str("</tr>\n");
            }
            Event::Start(Tag::TableCell) => {
                if in_table_head {
                    html_output.push_str("<th>");
                } else {
                    html_output.push_str("<td>");
                }
            }
            Event::End(TagEnd::TableCell) => {
                if in_table_head {
                    html_output.push_str("</th>\n");
                } else {
                    html_output.push_str("</td>\n");
                }
            }
            Event::Start(Tag::Heading { level, .. }) => {
                let level_num = heading_level_to_u8(level);
                html_output.push_str(&format!("<h{}>", level_num));
            }
            Event::End(TagEnd::Heading(level)) => {
                let level_num = heading_level_to_u8(level);
                html_output.push_str(&format!("</h{}>\n", level_num));
            }
            Event::Start(Tag::Paragraph) => {
                html_output.push_str("<p>");
            }
            Event::End(TagEnd::Paragraph) => {
                html_output.push_str("</p>\n");
            }
            Event::Start(Tag::List(None)) => {
                html_output.push_str("<ul>\n");
            }
            Event::Start(Tag::List(Some(start))) => {
                html_output.push_str(&format!("<ol start=\"{}\">\n", start));
            }
            Event::End(TagEnd::List(ordered)) => {
                if ordered {
                    html_output.push_str("</ol>\n");
                } else {
                    html_output.push_str("</ul>\n");
                }
            }
            Event::Start(Tag::Item) => {
                html_output.push_str("<li>");
            }
            Event::End(TagEnd::Item) => {
                html_output.push_str("</li>\n");
            }
            Event::Start(Tag::BlockQuote(_)) => {
                html_output.push_str("<blockquote>\n");
            }
            Event::End(TagEnd::BlockQuote(_)) => {
                html_output.push_str("</blockquote>\n");
            }
            Event::Start(Tag::Emphasis) => {
                html_output.push_str("<em>");
            }
            Event::End(TagEnd::Emphasis) => {
                html_output.push_str("</em>");
            }
            Event::Start(Tag::Strong) => {
                html_output.push_str("<strong>");
            }
            Event::End(TagEnd::Strong) => {
                html_output.push_str("</strong>");
            }
            Event::Start(Tag::Strikethrough) => {
                html_output.push_str("<del>");
            }
            Event::End(TagEnd::Strikethrough) => {
                html_output.push_str("</del>");
            }
            Event::Start(Tag::Link { dest_url, title, .. }) => {
                html_output.push_str(&format!(
                    "<a href=\"{}\" title=\"{}\">",
                    html_escape(&dest_url),
                    html_escape(&title)
                ));
            }
            Event::End(TagEnd::Link) => {
                html_output.push_str("</a>");
            }
            Event::Start(Tag::Image { dest_url, title, .. }) => {
                html_output.push_str(&format!(
                    "<img src=\"{}\" alt=\"",
                    html_escape(&dest_url)
                ));
                // The alt text will come as a Text event
                let _ = title;
            }
            Event::End(TagEnd::Image) => {
                html_output.push_str("\" />");
            }
            Event::Code(code) => {
                html_output.push_str("<code>");
                html_output.push_str(&html_escape(&code));
                html_output.push_str("</code>");
            }
            Event::Text(text) => {
                html_output.push_str(&html_escape(&text));
            }
            Event::SoftBreak => {
                html_output.push('\n');
            }
            Event::HardBreak => {
                html_output.push_str("<br />\n");
            }
            Event::Rule => {
                html_output.push_str("<hr />\n");
            }
            Event::TaskListMarker(checked) => {
                if checked {
                    html_output.push_str("<input type=\"checkbox\" checked disabled /> ");
                } else {
                    html_output.push_str("<input type=\"checkbox\" disabled /> ");
                }
            }
            _ => {}
        }
    }

    html_output
}

/// Escape HTML special characters
fn html_escape(text: &str) -> String {
    text.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
        .replace('\'', "&#39;")
}

/// Convert HeadingLevel enum to u8
fn heading_level_to_u8(level: HeadingLevel) -> u8 {
    match level {
        HeadingLevel::H1 => 1,
        HeadingLevel::H2 => 2,
        HeadingLevel::H3 => 3,
        HeadingLevel::H4 => 4,
        HeadingLevel::H5 => 5,
        HeadingLevel::H6 => 6,
    }
}

/// Generate @font-face CSS rules with embedded base64 fonts
fn generate_font_css() -> String {
    format!(
        r#"
/* Embedded Fonts */
@font-face {{
    font-family: 'Inter';
    font-style: normal;
    font-weight: 400;
    font-display: swap;
    src: url(data:font/woff2;base64,{}) format('woff2');
}}
@font-face {{
    font-family: 'Inter';
    font-style: normal;
    font-weight: 500;
    font-display: swap;
    src: url(data:font/woff2;base64,{}) format('woff2');
}}
@font-face {{
    font-family: 'Inter';
    font-style: normal;
    font-weight: 600;
    font-display: swap;
    src: url(data:font/woff2;base64,{}) format('woff2');
}}
@font-face {{
    font-family: 'Inter';
    font-style: normal;
    font-weight: 700;
    font-display: swap;
    src: url(data:font/woff2;base64,{}) format('woff2');
}}
@font-face {{
    font-family: 'Inter';
    font-style: normal;
    font-weight: 800;
    font-display: swap;
    src: url(data:font/woff2;base64,{}) format('woff2');
}}
@font-face {{
    font-family: 'JetBrains Mono';
    font-style: normal;
    font-weight: 400;
    font-display: swap;
    src: url(data:font/woff2;base64,{}) format('woff2');
}}
@font-face {{
    font-family: 'JetBrains Mono';
    font-style: normal;
    font-weight: 500;
    font-display: swap;
    src: url(data:font/woff2;base64,{}) format('woff2');
}}
"#,
        STANDARD.encode(INTER_REGULAR),
        STANDARD.encode(INTER_MEDIUM),
        STANDARD.encode(INTER_SEMIBOLD),
        STANDARD.encode(INTER_BOLD),
        STANDARD.encode(INTER_EXTRABOLD),
        STANDARD.encode(JETBRAINS_MONO_REGULAR),
        STANDARD.encode(JETBRAINS_MONO_MEDIUM),
    )
}

/// Generate full HTML document with CSS styling
fn generate_styled_html(content: &str, theme: &str) -> String {
    let font_css = generate_font_css();
    let theme_css = get_theme_css(theme);

    format!(
        r#"<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
{font_css}
{theme_css}
    </style>
</head>
<body>
    <article class="markdown-body">
{content}
    </article>
</body>
</html>"#
    )
}

/// Get CSS based on theme
fn get_theme_css(theme: &str) -> &'static str {
    match theme {
        "dark" => DARK_THEME_CSS,
        _ => LIGHT_THEME_CSS,
    }
}

/// Convert HTML to PDF using headless Chrome
fn html_to_pdf(html: &str, output_path: &str, page_size: &str) -> Result<(), String> {
    // Write HTML to a temp file (more reliable than data URLs for large content)
    let temp_dir = std::env::temp_dir();
    let temp_html_path = temp_dir.join(format!("md2pdf_{}.html", std::process::id()));
    fs::write(&temp_html_path, html)
        .map_err(|e| format!("Failed to write temp HTML file: {}", e))?;

    let browser = Browser::new(
        LaunchOptions::default_builder()
            .headless(true)
            .sandbox(false)
            .build()
            .map_err(|e| format!("Failed to build launch options: {}", e))?,
    )
    .map_err(|e| format!("Failed to launch browser: {}", e))?;

    let tab = browser
        .new_tab()
        .map_err(|e| format!("Failed to create new tab: {}", e))?;

    // Navigate to the temp HTML file
    let file_url = format!("file://{}", temp_html_path.display());

    tab.navigate_to(&file_url)
        .map_err(|e| format!("Failed to navigate: {}", e))?;

    tab.wait_until_navigated()
        .map_err(|e| format!("Failed to wait for navigation: {}", e))?;

    // Wait for page to fully render (important for embedded fonts to load)
    std::thread::sleep(std::time::Duration::from_millis(500));

    // Get page dimensions based on page size (in inches)
    let (paper_width, paper_height) = match page_size {
        "Letter" => (8.5, 11.0),
        _ => (8.27, 11.69), // A4 default
    };

    // Generate PDF with custom options
    let options = PrintToPdfOptions {
        landscape: Some(false),
        display_header_footer: Some(false),
        print_background: Some(true),
        scale: Some(1.0),
        paper_width: Some(paper_width),
        paper_height: Some(paper_height),
        margin_top: Some(0.5),
        margin_bottom: Some(0.5),
        margin_left: Some(0.5),
        margin_right: Some(0.5),
        prefer_css_page_size: Some(false),
        ..Default::default()
    };

    let pdf_bytes = tab
        .print_to_pdf(Some(options))
        .map_err(|e| format!("Failed to generate PDF: {}", e))?;

    // Ensure output directory exists
    if let Some(parent) = Path::new(output_path).parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create output directory: {}", e))?;
    }

    // Write PDF to file
    fs::write(output_path, pdf_bytes).map_err(|e| format!("Failed to write PDF: {}", e))?;

    // Clean up temp file
    let _ = fs::remove_file(&temp_html_path);

    Ok(())
}

// Light theme CSS - Professional document styling
const LIGHT_THEME_CSS: &str = r#"
@page {
    margin: 0;
    size: auto;
}

*, *::before, *::after {
    box-sizing: border-box;
}

html {
    font-size: 15px;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
    text-rendering: optimizeLegibility;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
}

body {
    font-family: "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif,
        "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei",
        "Hiragino Kaku Gothic Pro", "Yu Gothic",
        "Apple SD Gothic Neo", "Malgun Gothic",
        "Apple Color Emoji", "Segoe UI Emoji";
    font-size: 1rem;
    font-weight: 400;
    line-height: 1.7;
    color: #1a1a2e;
    background-color: #ffffff;
    margin: 0;
    padding: 0;
    word-wrap: break-word;
    font-feature-settings: "kern" 1, "liga" 1, "calt" 1;
}

.markdown-body {
    max-width: 100%;
    margin: 0 auto;
    padding: 56px 64px;
}

/* ==================== Typography ==================== */

/* Headings */
h1, h2, h3, h4, h5, h6 {
    font-weight: 700;
    line-height: 1.35;
    color: #0f0f23;
    margin-top: 2em;
    margin-bottom: 0.8em;
    letter-spacing: -0.02em;
    page-break-after: avoid;
    page-break-inside: avoid;
}

h1:first-child, h2:first-child, h3:first-child,
h4:first-child, h5:first-child, h6:first-child {
    margin-top: 0;
}

h1 {
    font-size: 2.4rem;
    font-weight: 800;
    letter-spacing: -0.03em;
    color: #0a0a1a;
    padding-bottom: 0.5em;
    margin-bottom: 1.2em;
    border-bottom: 3px solid #e8e8f0;
}

h2 {
    font-size: 1.8rem;
    font-weight: 700;
    color: #16163a;
    padding-bottom: 0.4em;
    margin-bottom: 1em;
    border-bottom: 2px solid #ececf4;
}

h3 {
    font-size: 1.4rem;
    font-weight: 600;
    color: #1f1f4a;
}

h4 {
    font-size: 1.15rem;
    font-weight: 600;
    color: #2a2a5a;
}

h5 {
    font-size: 1rem;
    font-weight: 600;
    color: #3a3a6a;
    text-transform: uppercase;
    letter-spacing: 0.04em;
}

h6 {
    font-size: 0.9rem;
    font-weight: 600;
    color: #5a5a8a;
    text-transform: uppercase;
    letter-spacing: 0.04em;
}

/* Paragraphs */
p {
    margin-top: 0;
    margin-bottom: 1.35em;
    line-height: 1.75;
}

/* Links */
a {
    color: #2563eb;
    text-decoration: none;
    border-bottom: 1px solid transparent;
    transition: border-color 0.15s ease;
}

a:hover {
    border-bottom-color: #2563eb;
}

/* ==================== Code ==================== */

/* Inline code */
code {
    font-family: "JetBrains Mono", "Fira Code", ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
    font-size: 0.88em;
    font-weight: 500;
    padding: 0.2em 0.45em;
    background: linear-gradient(135deg, #f8f9fc 0%, #f1f3f8 100%);
    border-radius: 5px;
    color: #c41d7f;
    border: 1px solid #e4e7ee;
    white-space: nowrap;
}

/* Code blocks - syntect generates pre with inline styles */
pre {
    font-family: "JetBrains Mono", "Fira Code", ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
    font-size: 0.88rem;
    line-height: 1.65;
    padding: 1.3em 1.5em;
    overflow-x: auto;
    background: linear-gradient(180deg, #fafbfd 0%, #f5f7fa 100%) !important;
    border-radius: 10px;
    border: 1px solid #e2e6ee;
    margin-top: 0;
    margin-bottom: 1.6em;
    page-break-inside: avoid;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

pre code {
    font-size: inherit;
    font-weight: 400;
    padding: 0;
    background: transparent !important;
    border: none;
    border-radius: 0;
    color: inherit;
    white-space: pre;
}

/* ==================== Blockquotes ==================== */

blockquote {
    margin: 0 0 1.5em 0;
    padding: 1em 1.5em;
    color: #4a5568;
    border-left: 4px solid #6366f1;
    background: linear-gradient(135deg, #f8f9ff 0%, #f3f4fc 100%);
    border-radius: 0 8px 8px 0;
    font-style: italic;
}

blockquote p {
    margin-bottom: 0.6em;
}

blockquote p:last-child {
    margin-bottom: 0;
}

blockquote blockquote {
    margin-top: 0.8em;
    border-left-color: #a5b4fc;
}

blockquote code {
    font-style: normal;
}

/* ==================== Lists ==================== */

ul, ol {
    margin-top: 0;
    margin-bottom: 1.4em;
    padding-left: 1.8em;
}

ul ul, ol ol, ul ol, ol ul {
    margin-bottom: 0;
    margin-top: 0.4em;
}

li {
    margin-bottom: 0.45em;
    line-height: 1.7;
}

li > p {
    margin-bottom: 0.6em;
}

li > p:last-child {
    margin-bottom: 0;
}

/* Custom bullet styling */
ul {
    list-style: none;
}

ul > li {
    position: relative;
    padding-left: 0.2em;
}

ul > li::before {
    content: "";
    position: absolute;
    left: -1.3em;
    top: 0.65em;
    width: 6px;
    height: 6px;
    background-color: #6366f1;
    border-radius: 50%;
}

ul ul > li::before {
    background-color: transparent;
    border: 1.5px solid #6366f1;
}

ul ul ul > li::before {
    background-color: #a5b4fc;
    border: none;
    width: 5px;
    height: 5px;
}

ol {
    list-style: none;
    counter-reset: ol-counter;
}

ol > li {
    position: relative;
    padding-left: 0.3em;
    counter-increment: ol-counter;
}

ol > li::before {
    content: counter(ol-counter) ".";
    position: absolute;
    left: -1.8em;
    top: 0;
    font-weight: 600;
    font-size: 0.9em;
    color: #6366f1;
    min-width: 1.5em;
    text-align: right;
}

/* Task lists */
li input[type="checkbox"] {
    margin-right: 0.6em;
    margin-left: -0.2em;
    vertical-align: middle;
    position: relative;
    top: -1px;
    width: 16px;
    height: 16px;
    accent-color: #6366f1;
}

/* ==================== Tables ==================== */

table {
    border-spacing: 0;
    border-collapse: separate;
    border-radius: 10px;
    margin-top: 0;
    margin-bottom: 1.8em;
    width: 100%;
    overflow: hidden;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
    page-break-inside: avoid;
}

thead {
    display: table-header-group;
}

tbody {
    display: table-row-group;
}

th, td {
    padding: 0.85em 1.1em;
    text-align: left;
    border-bottom: 1px solid #e8ebf0;
}

th {
    font-weight: 600;
    font-size: 0.9em;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: #4a5080;
    background: linear-gradient(180deg, #f8f9fc 0%, #f1f3f8 100%);
    border-bottom: 2px solid #dde0e8;
}

td {
    color: #2d3748;
}

tr:last-child td {
    border-bottom: none;
}

tbody tr:nth-child(even) {
    background-color: #fafbfd;
}

tbody tr:hover {
    background-color: #f5f6fa;
}

/* ==================== Other Elements ==================== */

/* Horizontal rule */
hr {
    height: 0;
    padding: 0;
    margin: 2.5em 0;
    border: 0;
    border-top: 2px solid #e8ebf0;
    background: transparent;
}

/* Images */
img {
    max-width: 100%;
    height: auto;
    display: block;
    margin: 1.5em auto;
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

/* Strikethrough */
del {
    color: #718096;
    text-decoration: line-through;
    text-decoration-color: #cbd5e0;
}

/* Strong and emphasis */
strong {
    font-weight: 650;
    color: #0f0f23;
}

em {
    font-style: italic;
    color: #2d3748;
}

/* Definition lists */
dt {
    font-weight: 600;
    margin-top: 1.2em;
    color: #1a1a2e;
}

dd {
    margin-left: 1.8em;
    margin-bottom: 0.6em;
    color: #4a5568;
}

/* Footnotes */
.footnote-definition {
    font-size: 0.88rem;
    margin-top: 2.5em;
    padding-top: 1.2em;
    border-top: 2px solid #e8ebf0;
    color: #4a5568;
}

/* Keyboard shortcut styling */
kbd {
    font-family: inherit;
    font-size: 0.85em;
    padding: 0.15em 0.4em;
    background: linear-gradient(180deg, #fff 0%, #f5f5f5 100%);
    border: 1px solid #d1d5db;
    border-radius: 4px;
    box-shadow: 0 1px 2px rgba(0,0,0,0.08), inset 0 -1px 0 rgba(0,0,0,0.1);
}

/* ==================== Print Optimizations ==================== */

@media print {
    html {
        font-size: 14px;
    }

    body {
        background: white;
    }

    .markdown-body {
        padding: 40px 48px;
    }

    pre, blockquote, table, img, h1, h2, h3, h4, h5, h6 {
        page-break-inside: avoid;
    }

    h1, h2, h3, h4, h5, h6 {
        page-break-after: avoid;
    }

    p, li {
        orphans: 3;
        widows: 3;
    }

    a {
        color: #2563eb;
    }

    a[href^="http"]::after {
        content: " (" attr(href) ")";
        font-size: 0.8em;
        color: #718096;
        word-break: break-all;
    }

    table {
        box-shadow: none;
        border: 1px solid #d1d5db;
    }

    img {
        box-shadow: none;
        border: 1px solid #e8ebf0;
    }

    pre {
        box-shadow: none;
        border: 1px solid #d1d5db;
    }
}
"#;

// Dark theme CSS - Professional dark document styling
const DARK_THEME_CSS: &str = r#"
@page {
    margin: 0;
    size: auto;
}

*, *::before, *::after {
    box-sizing: border-box;
}

html {
    font-size: 15px;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
    text-rendering: optimizeLegibility;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
}

body {
    font-family: "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif,
        "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei",
        "Hiragino Kaku Gothic Pro", "Yu Gothic",
        "Apple SD Gothic Neo", "Malgun Gothic",
        "Apple Color Emoji", "Segoe UI Emoji";
    font-size: 1rem;
    font-weight: 400;
    line-height: 1.7;
    color: #e2e8f0;
    background-color: #0f172a;
    margin: 0;
    padding: 0;
    word-wrap: break-word;
    font-feature-settings: "kern" 1, "liga" 1, "calt" 1;
}

.markdown-body {
    max-width: 100%;
    margin: 0 auto;
    padding: 56px 64px;
}

/* ==================== Typography ==================== */

/* Headings */
h1, h2, h3, h4, h5, h6 {
    font-weight: 700;
    line-height: 1.35;
    color: #f1f5f9;
    margin-top: 2em;
    margin-bottom: 0.8em;
    letter-spacing: -0.02em;
    page-break-after: avoid;
    page-break-inside: avoid;
}

h1:first-child, h2:first-child, h3:first-child,
h4:first-child, h5:first-child, h6:first-child {
    margin-top: 0;
}

h1 {
    font-size: 2.4rem;
    font-weight: 800;
    letter-spacing: -0.03em;
    color: #f8fafc;
    padding-bottom: 0.5em;
    margin-bottom: 1.2em;
    border-bottom: 3px solid #334155;
}

h2 {
    font-size: 1.8rem;
    font-weight: 700;
    color: #f1f5f9;
    padding-bottom: 0.4em;
    margin-bottom: 1em;
    border-bottom: 2px solid #1e293b;
}

h3 {
    font-size: 1.4rem;
    font-weight: 600;
    color: #e2e8f0;
}

h4 {
    font-size: 1.15rem;
    font-weight: 600;
    color: #cbd5e1;
}

h5 {
    font-size: 1rem;
    font-weight: 600;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: 0.04em;
}

h6 {
    font-size: 0.9rem;
    font-weight: 600;
    color: #64748b;
    text-transform: uppercase;
    letter-spacing: 0.04em;
}

/* Paragraphs */
p {
    margin-top: 0;
    margin-bottom: 1.35em;
    line-height: 1.75;
}

/* Links */
a {
    color: #60a5fa;
    text-decoration: none;
    border-bottom: 1px solid transparent;
    transition: border-color 0.15s ease;
}

a:hover {
    border-bottom-color: #60a5fa;
}

/* ==================== Code ==================== */

/* Inline code */
code {
    font-family: "JetBrains Mono", "Fira Code", ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
    font-size: 0.88em;
    font-weight: 500;
    padding: 0.2em 0.45em;
    background: linear-gradient(135deg, #1e293b 0%, #1a2332 100%);
    border-radius: 5px;
    color: #f472b6;
    border: 1px solid #334155;
    white-space: nowrap;
}

/* Code blocks - syntect generates pre with inline styles */
pre {
    font-family: "JetBrains Mono", "Fira Code", ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
    font-size: 0.88rem;
    line-height: 1.65;
    padding: 1.3em 1.5em;
    overflow-x: auto;
    background: linear-gradient(180deg, #1e293b 0%, #172033 100%) !important;
    border-radius: 10px;
    border: 1px solid #334155;
    margin-top: 0;
    margin-bottom: 1.6em;
    page-break-inside: avoid;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
}

pre code {
    font-size: inherit;
    font-weight: 400;
    padding: 0;
    background: transparent !important;
    border: none;
    border-radius: 0;
    color: inherit;
    white-space: pre;
}

/* ==================== Blockquotes ==================== */

blockquote {
    margin: 0 0 1.5em 0;
    padding: 1em 1.5em;
    color: #94a3b8;
    border-left: 4px solid #818cf8;
    background: linear-gradient(135deg, #1e293b 0%, #1a2438 100%);
    border-radius: 0 8px 8px 0;
    font-style: italic;
}

blockquote p {
    margin-bottom: 0.6em;
}

blockquote p:last-child {
    margin-bottom: 0;
}

blockquote blockquote {
    margin-top: 0.8em;
    border-left-color: #6366f1;
}

blockquote code {
    font-style: normal;
}

/* ==================== Lists ==================== */

ul, ol {
    margin-top: 0;
    margin-bottom: 1.4em;
    padding-left: 1.8em;
}

ul ul, ol ol, ul ol, ol ul {
    margin-bottom: 0;
    margin-top: 0.4em;
}

li {
    margin-bottom: 0.45em;
    line-height: 1.7;
}

li > p {
    margin-bottom: 0.6em;
}

li > p:last-child {
    margin-bottom: 0;
}

/* Custom bullet styling */
ul {
    list-style: none;
}

ul > li {
    position: relative;
    padding-left: 0.2em;
}

ul > li::before {
    content: "";
    position: absolute;
    left: -1.3em;
    top: 0.65em;
    width: 6px;
    height: 6px;
    background-color: #818cf8;
    border-radius: 50%;
}

ul ul > li::before {
    background-color: transparent;
    border: 1.5px solid #818cf8;
}

ul ul ul > li::before {
    background-color: #6366f1;
    border: none;
    width: 5px;
    height: 5px;
}

ol {
    list-style: none;
    counter-reset: ol-counter;
}

ol > li {
    position: relative;
    padding-left: 0.3em;
    counter-increment: ol-counter;
}

ol > li::before {
    content: counter(ol-counter) ".";
    position: absolute;
    left: -1.8em;
    top: 0;
    font-weight: 600;
    font-size: 0.9em;
    color: #818cf8;
    min-width: 1.5em;
    text-align: right;
}

/* Task lists */
li input[type="checkbox"] {
    margin-right: 0.6em;
    margin-left: -0.2em;
    vertical-align: middle;
    position: relative;
    top: -1px;
    width: 16px;
    height: 16px;
    accent-color: #818cf8;
}

/* ==================== Tables ==================== */

table {
    border-spacing: 0;
    border-collapse: separate;
    border-radius: 10px;
    margin-top: 0;
    margin-bottom: 1.8em;
    width: 100%;
    overflow: hidden;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
    page-break-inside: avoid;
}

thead {
    display: table-header-group;
}

tbody {
    display: table-row-group;
}

th, td {
    padding: 0.85em 1.1em;
    text-align: left;
    border-bottom: 1px solid #334155;
}

th {
    font-weight: 600;
    font-size: 0.9em;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: #94a3b8;
    background: linear-gradient(180deg, #1e293b 0%, #172033 100%);
    border-bottom: 2px solid #475569;
}

td {
    color: #cbd5e1;
}

tr:last-child td {
    border-bottom: none;
}

tbody tr:nth-child(even) {
    background-color: rgba(30, 41, 59, 0.5);
}

tbody tr:hover {
    background-color: rgba(51, 65, 85, 0.4);
}

/* ==================== Other Elements ==================== */

/* Horizontal rule */
hr {
    height: 0;
    padding: 0;
    margin: 2.5em 0;
    border: 0;
    border-top: 2px solid #334155;
    background: transparent;
}

/* Images */
img {
    max-width: 100%;
    height: auto;
    display: block;
    margin: 1.5em auto;
    border-radius: 8px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.35);
}

/* Strikethrough */
del {
    color: #64748b;
    text-decoration: line-through;
    text-decoration-color: #475569;
}

/* Strong and emphasis */
strong {
    font-weight: 650;
    color: #f1f5f9;
}

em {
    font-style: italic;
    color: #cbd5e1;
}

/* Definition lists */
dt {
    font-weight: 600;
    margin-top: 1.2em;
    color: #e2e8f0;
}

dd {
    margin-left: 1.8em;
    margin-bottom: 0.6em;
    color: #94a3b8;
}

/* Footnotes */
.footnote-definition {
    font-size: 0.88rem;
    margin-top: 2.5em;
    padding-top: 1.2em;
    border-top: 2px solid #334155;
    color: #94a3b8;
}

/* Keyboard shortcut styling */
kbd {
    font-family: inherit;
    font-size: 0.85em;
    padding: 0.15em 0.4em;
    background: linear-gradient(180deg, #334155 0%, #1e293b 100%);
    border: 1px solid #475569;
    border-radius: 4px;
    color: #e2e8f0;
    box-shadow: 0 1px 2px rgba(0,0,0,0.3), inset 0 -1px 0 rgba(0,0,0,0.2);
}

/* ==================== Print Optimizations ==================== */

@media print {
    html {
        font-size: 14px;
    }

    body {
        background: #0f172a;
    }

    .markdown-body {
        padding: 40px 48px;
    }

    pre, blockquote, table, img, h1, h2, h3, h4, h5, h6 {
        page-break-inside: avoid;
    }

    h1, h2, h3, h4, h5, h6 {
        page-break-after: avoid;
    }

    p, li {
        orphans: 3;
        widows: 3;
    }

    a {
        color: #60a5fa;
    }

    a[href^="http"]::after {
        content: " (" attr(href) ")";
        font-size: 0.8em;
        color: #64748b;
        word-break: break-all;
    }

    table {
        box-shadow: none;
        border: 1px solid #475569;
    }

    img {
        box-shadow: none;
        border: 1px solid #334155;
    }

    pre {
        box-shadow: none;
        border: 1px solid #475569;
    }
}
"#;
