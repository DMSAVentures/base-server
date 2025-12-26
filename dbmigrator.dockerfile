# Use the official Flyway Docker image
FROM flyway/flyway:latest

# Switch to the root user to modify permissions
USER root

# Copy migration files into Flyway's sql folder
COPY ./migrations /flyway/sql

# Copy the entrypoint script and make it executable
COPY entrypoint.sh /flyway/entrypoint.sh
RUN chmod +x /flyway/entrypoint.sh

# Set the script as the entrypoint
ENTRYPOINT ["/flyway/entrypoint.sh"]
