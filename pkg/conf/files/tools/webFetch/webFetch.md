Fetches content from the web and returns it either as raw text or converted Markdown.

Usage:
- Provide `url` with an `http` or `https` scheme.
- Set `format` to:
  - `raw` to return response body without modifications.
  - `markdown` to convert HTML responses to Markdown.
- `format` is optional. If not provided, it defaults to "markdown" for HTML content, or "raw" for other textual content types (e.g., text/plain, application/json).
- `timeout` is optional and defaults to 30 seconds.

Notes:
- Markdown conversion currently supports only HTML content.
- If format is not specified and the content type is non-textual (e.g., binary), an error will be returned.
- The result includes `content`, `statusCode`, `contentType`, `url`, and `format`.
