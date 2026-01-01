BARGE ?= $(LOCALBIN)/barge

.PHONY: barge
barge: $(BARGE)
$(BARGE): $(LOCALBIN)
	@dagger call binary export --path $(BARGE)

.PHONY: .git/hooks .git/hooks/ .git/hooks/pre-commit
.git/hooks .git/hooks/ .git/hooks/pre-commit:
	@cp .githooks/* .git/hooks

.PHONY: release
release:
	@git tag $(SEMVER)
	@git push
	@git push --tags

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

BIN ?= ~/.local/bin
INSTALL ?= install

.PHONY: install
install: barge
	@$(INSTALL) $(BARGE) $(BIN)
