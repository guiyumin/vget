package extractor

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bilibili title with CJK brackets and special chars",
			input:    "【这还不薅吗？】阿里云史低神价：35元/年，197.7元/5年！395.4元解锁10年\u201c超值传家宝\u201d，享200Mbps带宽+隐藏福利！哇！哇！哇！",
			expected: "这还不薅吗阿里云史低神价-35元-年，197.7元-5年！395.4元解锁10年\u201c超值传家宝\u201d，享200Mbps带宽+隐", // truncated to 60 runes
		},
		{
			name:     "ASCII reserved characters",
			input:    "test:file*name?with<special>chars|here",
			expected: "test-filenamewithspecialcharshere",
		},
		{
			name:     "Full-width reserved characters",
			input:    "测试：文件＊名？含＜特殊＞字符｜这里",
			expected: "测试-文件名含特殊字符这里",
		},
		{
			name:     "Path separators",
			input:    "path/to\\file",
			expected: "path-to-file",
		},
		{
			name:     "Full-width path separators",
			input:    "路径／到＼文件",
			expected: "路径-到-文件",
		},
		{
			name:     "CJK brackets",
			input:    "【标题】「内容」",
			expected: "标题内容",
		},
		{
			name:     "Windows reserved name CON",
			input:    "CON",
			expected: "_CON",
		},
		{
			name:     "Windows reserved name lowercase",
			input:    "aux",
			expected: "_aux",
		},
		{
			name:     "Windows reserved name COM1",
			input:    "COM1",
			expected: "_COM1",
		},
		{
			name:     "Trailing dots and spaces",
			input:    "filename...",
			expected: "filename",
		},
		{
			name:     "Multiple spaces",
			input:    "file   name   here",
			expected: "file name here",
		},
		{
			name:     "Control characters",
			input:    "file\x00name\x1fhere",
			expected: "filenamehere",
		},
		{
			name:     "Newlines and tabs",
			input:    "file\nname\there",
			expected: "file name here",
		},
		{
			name:     "URL in title",
			input:    "Check out https://example.com/path for more",
			expected: "Check out for more",
		},
		{
			name:     "Long filename truncation",
			input:    "这是一个非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常长的标题",
			expected: "这是一个非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常", // 60 runes
		},
		{
			name:     "Empty after sanitization",
			input:    "???***",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFilename(%q)\n  got:  %q\n  want: %q", tt.input, result, tt.expected)
			}
		})
	}
}
