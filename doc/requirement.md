# HTTP MCP Server Implementation

## Project Overview
This project implements an HTTP MCP (Model Context Protocol) server with the following modifications:
1. Including a header parameter "Mcp-Session-Id" to track specific user requests
2. Generating a unique Mcp-Session-Id that will be sent to clients initiating connections
3. Validating that clients send the Mcp-Session-Id parameter back in their responses
4. Maintaining all active Mcp-Session-Ids for validation purposes
5. Following the MCP documentation to ensure compliance with HTTP MCP server specifications
6. Supporting the required MCP endpoints: /mcp, /mcp/send, /mcp/receive, and /mcp/abort
7. Implementing proper error handling with appropriate HTTP status codes
8. Supporting the required Content-Type header (application/json)
9. Implementing proper message sequencing and validation
10. Supporting the required JSON schema for requests and responses
11. Handling timeouts and connection management as specified in the MCP documentation
12. Implementing proper versioning support via the Mcp-Version header

## Documentation
For detailed specifications, please refer to the official MCP documentation:
[https://modelcontextprotocol.io/llms-full.txt](https://modelcontextprotocol.io/llms-full.txt)
