# Leaderboard Service - CNCF-Compliant Structure

This project has been restructured following the [Cloud Native Computing Foundation (CNCF)](https://www.cncf.io/) recommended project layout for Go applications.

## Project Structure

```
.
├── cmd/                # Command-line applications
│   └── leaderboard/    # Main application entry point
│       └── main.go     # Main application code
├── internal/           # Private application code
│   ├── api/            # API handlers and routes
│   ├── config/         # Configuration management
│   ├── db/             # Database access and persistence
│   ├── docs/           # Swagger documentation
│   └── models/         # Data models
├── ...                 # Other project files and directories
```

## Design Philosophy

The project follows these key CNCF design principles:

1. **Separation of Concerns**: Code is organized by functionality in distinct packages
2. **Encapsulation**: Private application code is in the `internal/` directory
3. **Scalability**: The structure allows for multiple applications under `cmd/`
4. **Maintainability**: Clear organization makes the codebase easier to understand and modify

## Migration Notes

This project was migrated from a flat structure to the CNCF-compliant structure. The migration involved:

1. Moving the main application code to `cmd/leaderboard/main.go`
2. Moving core packages to the `internal/` directory
3. Updating import paths throughout the codebase

## Building and Running

The application can be built and run using the included Makefile:

```bash
# Build the application
make build

# Run the application
make run

# Run with hot reloading for development
make dev
```

## Further Improvements

Further improvements could include:

1. Fully removing the original package directories once all imports are updated
2. Adding more applications under the `cmd/` directory if needed
3. Extracting shared packages that might be useful outside this project into a `pkg/` directory 