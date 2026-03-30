docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d
docker-down:
	docker compose -f deployments/docker/docker-compose.yml down
proto-gen:
	mkdir -p internal/transport/pb
	protoc \
		--proto_path=./proto \
		--go_out=./internal/transport/pb \
		--go_opt=paths=source_relative \
		--go-grpc_out=./internal/transport/pb \
		--go-grpc_opt=paths=source_relative \
		proto/*.proto
		