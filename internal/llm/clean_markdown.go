package llm

import (
	"context"
	"fmt"
	"log/slog"
)

const cleanMarkdownPrompt = `Clean and enhance this markdown content following these strict rules:

CONTENT RULES:
1. Keep only information directly related to the main topic
2. Remove any promotional, advertising, or unrelated content
3. Remove navigation elements, footers, and sidebars
4. Keep code blocks and technical content if relevant
5. Preserve important quotes and key points

FORMATTING RULES:
1. Use proper markdown heading hierarchy (h1 -> h2 -> h3)
2. Ensure consistent spacing between sections
3. Fix or remove malformed markdown syntax
4. Convert HTML to markdown where possible
5. Remove redundant line breaks and spaces

IMAGE AND LINK RULES:
1. Keep only the most relevant and informative images
2. Remove decorative or redundant images
3. Remove broken or relative links
4. Remove duplicate links pointing to the same content
5. Keep essential reference links

CLEANUP RULES:
1. Remove empty sections
2. Remove non-English content unless it's code
3. Fix list formatting and indentation
4. Remove HTML comments and metadata
5. Remove social media embeds unless they're the main content

Content to clean:
%s
`

func (c *OpenAIClient) CleanMarkdown(content string) (string, error) {
	slog.Info("cleaning markdown", "model", c.model, "length", len(content))
	return c.callLLM(context.Background(), fmt.Sprintf("%s%s", cleanMarkdownPrompt, content))
}
