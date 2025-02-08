curl --request POST \
  --url https://api.firecrawl.dev/v1/scrape \
  --header 'Authorization: Bearer <token>' \
  --header 'Content-Type: application/json' \
  --data '{
  "url": "https://github.com/punkpeye/awesome-mcp-clients?utm_source=perplexity",
  "formats": [
    "markdown",
    "json"
  ],
  "onlyMainContent": true,
  "headers": {},
  "waitFor": 5,
  "mobile": false,
  "skipTlsVerification": false,
  "timeout": 30000,
  "jsonOptions": {
    "schema": {
      "type": "object",
      "properties": {
        "description": {
          "type": "string",
          "description": "Description of a website."
        },
        "category": {
          "type": "string",
          "description": "Single category of a website (ai/devops/linux/sysops)."
        },
        "tags": {
          "type": "array",
          "description": "List of tags.",
          "items": {
            "type": "string"
          }
        }
      },
      "required": [
        "description"
      ]
    }
  },
  "location": {
    "country": "US",
    "languages": [
      "en-US"
    ]
  },
  "removeBase64Images": true,
  "blockAds": true
}'
