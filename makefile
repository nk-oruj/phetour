build:
	go run ./source

deploy-html:
	ssh phetour "rm -rf ./www/html/*" && scp -r ./output/html/* phetour:~/www/html/

deploy-gmi:
	ssh phetour "rm -rf ./www/gmi/*" && scp -r ./output/gmi/* phetour:~/www/gmi/

deploy-all: deploy-html deploy-gmi

deploy-fast: build deploy-all
