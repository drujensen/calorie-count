# Calorie Count

A web application to help you track your daily calorie intake.

## Features

- Track daily calorie consumption
- Set calorie goals
- View progress and statistics
- Responsive web interface

## Getting Started

### Prerequisites

- Go 1.21 or later
- Node.js (for frontend development)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/calorie-count.git
   cd calorie-count
   ```

2. Install dependencies:
   ```bash
   make install
   ```

3. Run the application:
   ```bash
   make run
   ```

### Development

- Backend runs on `http://localhost:8080`
- Frontend runs on `http://localhost:3000`

## Project Structure

```
calorie-count/
├── cmd/                      # Application entry points
├── internal/                 # Private application code
│   ├── config/               # Configuration management
│   ├── handlers/             # HTTP handlers
│   ├── middleware/           # HTTP middleware
│   ├── models/               # Data models
│   ├── repositories/         # Data access layer
│   ├── services/             # Business logic
│   ├── templates/            # HTML templates
│   └── utils/                # Utility functions
├── web/                      # Frontend assets
├── tests/                    # Integration/E2E tests
└── ...
```

## License

MIT
