Use following command to migrate the database:
Step 1: Build the docker image
```run
docker build -t flyway-migrate -f dbmigrator.dockerfile .
```
Step 2: Connect to DB using AWS SSM
```run
aws ssm start-session \
  --target <instance-id> \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters "host=DB_HOST,portNumber=5432,localPortNumber=5432"
```
Step 3: Run the docker image
```
docker run --platform linux/amd64 --rm \
  -e DB_HOST=host.docker.internal \
  -e DB_USERNAME=username \
  -e DB_PASSWORD=password \
  flyway-migrate
```