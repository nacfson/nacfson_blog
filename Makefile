.PHONY: studio studio-ui studio-test

studio-ui:
	npm --prefix studio ci
	npm --prefix studio run build
	rm -rf internal/studio/ui/dist
	mkdir -p internal/studio/ui/dist
	cp -R studio/dist/. internal/studio/ui/dist/
	touch internal/studio/ui/dist/.keep

studio: studio-ui
	mkdir -p bin
	go build -o bin/blog-studio ./cmd/blog-studio

studio-test:
	go test -race ./internal/studio/...
	npm --prefix studio test
