# Authentication Service

This service manages user authentication. It exposes a `/login` endpoint that
accepts a JSON payload containing a username and password. On successful
authentication it returns a short lived token generated using the shared
`libs/auth` package. A simple in-memory repository is used for storing users and
is initialized with a default `admin` user when the service starts.
