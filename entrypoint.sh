#!/bin/sh
set -eu

# Set defaults for environment variables if missing
# POSTGRES_PASSWORD is set here (not in Dockerfile ENV) to avoid build-time security warnings
: "${PGDATA:=/var/lib/postgresql/data}"
: "${POSTGRES_DB:=lumescope}"
: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_PASSWORD:=postgres}"
: "${PORT:=18080}"
: "${DB_DSN:=postgres://postgres:postgres@localhost:5432/lumescope?sslmode=disable}"

# Export for child processes
export PGDATA POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD PORT DB_DSN

# Initialize database if not already initialized
if [ ! -f "$PGDATA/PG_VERSION" ]; then
    echo "Initializing PostgreSQL database..."
    mkdir -p "$PGDATA"
    chmod 700 "$PGDATA"
    
    # Create temporary password file
    tmp_passfile=$(mktemp)
    echo "$POSTGRES_PASSWORD" > "$tmp_passfile"
    
    # Initialize the database
    initdb -D "$PGDATA" -U "$POSTGRES_USER" --pwfile="$tmp_passfile"
    
    # Clean up password file
    rm -f "$tmp_passfile"
    
    echo "PostgreSQL database initialized."
fi

# Start PostgreSQL
echo "Starting PostgreSQL..."
pg_ctl -D "$PGDATA" -o "-c listen_addresses=*" -w start

# Create the database only if it doesn't exist
echo "Checking for database '$POSTGRES_DB'..."
if ! psql -U "$POSTGRES_USER" -lqt | cut -d \| -f 1 | grep -qw "$POSTGRES_DB"; then
    echo "Creating database '$POSTGRES_DB'..."
    createdb --username="$POSTGRES_USER" "$POSTGRES_DB"
else
    echo "Database '$POSTGRES_DB' already exists."
fi

# Signal handling
terminate() {
    echo "Shutting down..."
    pg_ctl -D "$PGDATA" -m fast -w stop
    exit 0
}

trap terminate INT TERM

# Run the application (from /app working directory so docs/openapi.json is found)
echo "Starting LumeScope on port $PORT..."
./lumescope &
APP_PID=$!

# Wait for the application to exit
wait $APP_PID
EXIT_CODE=$?

# Terminate PostgreSQL on app exit
terminate

exit $EXIT_CODE