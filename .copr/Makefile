.PHONY: srpm

SPEC := git-this-bread.spec

srpm:
ifndef VERSION
	$(error VERSION is required, e.g. make srpm VERSION=1.0.0 outdir=/tmp)
endif
	go mod vendor
	mkdir -p rpmbuild/SOURCES rpmbuild/SPECS
	tar czf rpmbuild/SOURCES/git-this-bread-$(VERSION).tar.gz \
		--transform 's,^\.,git-this-bread-$(VERSION),' \
		--exclude=.git --exclude=rpmbuild .
	sed 's/@@VERSION@@/$(VERSION)/g' $(SPEC) > rpmbuild/SPECS/$(SPEC)
	rpmbuild -bs rpmbuild/SPECS/$(SPEC) \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--define "_sourcedir $(CURDIR)/rpmbuild/SOURCES"
	mv rpmbuild/SRPMS/*.src.rpm $(outdir)
