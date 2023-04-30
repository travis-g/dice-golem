.PHONY: dev
dev:
	@bundle exec jekyll serve -H $$(ifconfig | grep "inet " | grep broadcast | cut -d\  -f2) --livereload
