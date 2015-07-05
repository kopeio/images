.PHONY: image push

default: image

image:
	docker build -t kope/java7 .

push: image
	docker push kope/java7
