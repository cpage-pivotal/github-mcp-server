# Deploying GitHub MCP Server to Cloud Foundry

This guide covers deploying the GitHub MCP Server to Cloud Foundry using the SSE (Server-Sent Events) transport.

## Prerequisites

1. **Cloud Foundry CLI** installed and configured
2. **GitHub Personal Access Token** with appropriate permissions
3. **Cloud Foundry account** with sufficient resources

## Deployment Steps

### 1. Build the Application

```bash
# Build the Go application
go build -o bin/github-mcp-server ./cmd/github-mcp-server
```

### 2. Configure Environment Variables

Update the `manifest.yml` file with your specific configuration:

```yaml
---
applications:
- name: github-mcp-server
  memory: 512M
  instances: 1
  buildpacks:
    - go_buildpack
  command: ./bin/github-mcp-server sse --gh-host=$GITHUB_HOST --toolsets=$GITHUB_TOOLSETS --read-only=$GITHUB_READ_ONLY --enable-command-logging=$GITHUB_ENABLE_COMMAND_LOGGING
  env:
    GITHUB_PERSONAL_ACCESS_TOKEN: ((github-token))
    GITHUB_HOST: github.com
    GITHUB_TOOLSETS: repositories,issues,pullrequests,search,notifications,code_scanning,secret_scanning,context_tools
    GITHUB_READ_ONLY: false
    GITHUB_ENABLE_COMMAND_LOGGING: true
    PORT: 8080
  services:
    - github-mcp-logs
  health-check-type: http
  health-check-http-endpoint: /health
  routes:
    - route: github-mcp-server.((domain))
```

### 3. Set Up Variables

Create a `vars.yml` file to define your deployment variables:

```yaml
github-token: your-github-personal-access-token
domain: your-cloud-foundry-domain.com
```

**Security Note**: Never commit your actual GitHub token to version control. Use Cloud Foundry's credential management or external secret stores.

### 4. Deploy to Cloud Foundry

```bash
# Deploy using the manifest
cf push --vars-file vars.yml

# Or deploy with inline variables
cf push --var github-token=your-token --var domain=your-domain.com
```

### 5. Verify Deployment

Check the application status:

```bash
# Check app status
cf apps

# View logs
cf logs github-mcp-server --recent

# Check health endpoint
curl https://github-mcp-server.your-domain.com/health
```

## SSE Endpoints

Once deployed, the server will expose these endpoints:

- **Health Check**: `https://your-app.your-domain.com/health`
- **SSE Stream**: `https://your-app.your-domain.com/github-mcp/sse`
- **Message Endpoint**: `https://your-app.your-domain.com/github-mcp/message?sessionId={sessionId}`

## Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `GITHUB_PERSONAL_ACCESS_TOKEN` | GitHub PAT for API access | - | Yes |
| `GITHUB_HOST` | GitHub hostname (github.com or enterprise) | github.com | No |
| `GITHUB_TOOLSETS` | Comma-separated list of enabled toolsets | all | No |
| `GITHUB_READ_ONLY` | Restrict to read-only operations | false | No |
| `GITHUB_ENABLE_COMMAND_LOGGING` | Enable request/response logging | false | No |
| `PORT` | HTTP port for the server | 8080 | No |

## Available Toolsets

- `repositories` - Repository management
- `issues` - Issue tracking
- `pullrequests` - Pull request operations
- `search` - GitHub search functionality
- `notifications` - Notification management
- `code_scanning` - Code scanning alerts
- `secret_scanning` - Secret scanning alerts
- `context_tools` - Context-aware tools

## Client Connection

To connect an MCP client to your deployed server:

```javascript
// Example client configuration
const client = new MCPClient({
  transport: {
    type: 'sse',
    url: 'https://github-mcp-server.your-domain.com/github-mcp'
  }
});
```

## Scaling and Production Considerations

### Memory and CPU

- **Memory**: Start with 512M, monitor usage and adjust as needed
- **Instances**: Begin with 1 instance, scale based on load
- **CPU**: Default CPU allocation is usually sufficient

### Logging

Configure structured logging for production:

```yaml
env:
  GITHUB_ENABLE_COMMAND_LOGGING: true
```

Logs will be available via `cf logs github-mcp-server`.

### Security

1. **Use credential services** for storing GitHub tokens
2. **Enable HTTPS** (handled by Cloud Foundry router)
3. **Restrict toolsets** if not all functionality is needed
4. **Consider read-only mode** for reduced risk

### Monitoring

Set up monitoring for:
- Application health (`/health` endpoint)
- Response times
- Error rates
- GitHub API rate limits

## Troubleshooting

### Common Issues

1. **Build Failures**
   ```bash
   # Check build logs
   cf logs github-mcp-server --recent
   ```

2. **Authentication Issues**
   - Verify GitHub token has correct permissions
   - Check token isn't expired

3. **Memory Issues**
   ```bash
   # Increase memory allocation
   cf scale github-mcp-server -m 1G
   ```

4. **Connection Issues**
   - Verify health endpoint is accessible
   - Check application routes: `cf routes`

### Debugging

Enable debug logging:

```yaml
env:
  GITHUB_ENABLE_COMMAND_LOGGING: true
```

View real-time logs:

```bash
cf logs github-mcp-server
```

## Advanced Configuration

### Custom Domain

To use a custom domain:

1. Map the domain: `cf map-route github-mcp-server your-domain.com`
2. Update DNS records
3. Configure SSL certificate

### Blue-Green Deployment

For zero-downtime deployments:

```bash
# Deploy to staging
cf push github-mcp-server-staging

# Test staging environment
# Then swap routes
cf map-route github-mcp-server-staging your-domain.com
cf unmap-route github-mcp-server your-domain.com
```

### Service Binding

If using external services (databases, caches):

```yaml
services:
  - github-mcp-logs
  - redis-cache
  - postgres-db
```

## Security Best Practices

1. **Rotate GitHub tokens regularly**
2. **Use least-privilege access** for toolsets
3. **Monitor for unusual API usage**
4. **Enable read-only mode** when possible
5. **Use Cloud Foundry security groups**

This completes the Cloud Foundry deployment setup for the GitHub MCP Server with SSE transport.