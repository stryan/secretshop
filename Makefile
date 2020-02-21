PREFIX?=/usr/local
_INSTDIR=$(DESTDIR)$(PREFIX)
BINDIR?=$(_INSTDIR)/bin
GO?=go
GOFLAGS?=

GOSRC!=find . -name '*.go'
GOSRC+=go.mod go.sum
RM?=rm -f

secretshop: $(GOSRC)
	$(GO) build $(GOFLAGS) \
		-ldflags "-s -w" \
		-o $@

all: secretshop

install: all
	mkdir -m755 -p $(BINDIR) /etc/secretshop
	install -m755 secretshop $(BINDIR)/secretshop
	install -m644 config.yaml.sample /etc/secretshop/
service: all install
	install -m644 secretshop.service /etc/systemd/system/
clean:
	$(RM) secretshop
RMDIR_IF_EMPTY:=sh -c '\
if test -d $$0 && ! ls -1qA $$0 | grep -q . ; then \
	rmdir $$0; \
fi'

uninstall:
	$(RM) $(BINDIR)/secretshop
	$(RM) /etc/systemd/system/secretshop.service
	${RMDIR_IF_EMPTY} /etc/secretshop

.DEFAULT_GOAL := all
.PHONY: all install service uninstall clean
