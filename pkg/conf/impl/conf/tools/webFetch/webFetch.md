Fetches content from the web and returns it either as raw text or converted Markdown.

Usage:
- Provide `url` with an `http` or `https` scheme.
- Set `format` to:
  - `raw` to return response body without modifications.
  - `markdown` to convert HTML responses to Markdown.
- `timeout` is optional and defaults to 30 seconds.

Notes:
- Markdown conversion currently supports only HTML content.
- The result includes `content`, `statusCode`, `contentType`, `url`, and `format`.
