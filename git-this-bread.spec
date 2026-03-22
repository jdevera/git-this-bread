%global goipath github.com/jdevera/git-this-bread
%global debug_package %{nil}

Name:           git-this-bread
Version:        @@VERSION@@
Release:        1%{?dist}
Summary:        Git utilities for developers who knead to understand their repos

License:        MIT
URL:            https://%{goipath}
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24

%description
A collection of git and GitHub CLI utilities: git-explain (repo status
analyzer), git-as/git-id/gh-as (identity management), and gh-wtfork
(fork analyzer).

%prep
%autosetup -n %{name}-%{version}

%build
LDFLAGS="-s -w -X main.version=%{version}"
for cmd in git-explain git-id git-as gh-as gh-wtfork; do
    GOFLAGS=-mod=vendor go build -ldflags "$LDFLAGS" -o "$cmd" "./cmd/$cmd"
done

%install
for cmd in git-explain git-id git-as gh-as gh-wtfork; do
    install -Dpm 0755 "$cmd" %{buildroot}%{_bindir}/"$cmd"
done

%files
%license LICENSE
%{_bindir}/git-explain
%{_bindir}/git-id
%{_bindir}/git-as
%{_bindir}/gh-as
%{_bindir}/gh-wtfork

%changelog
* Sun Mar 22 2026 Jacobo de Vera <73069+jdevera@users.noreply.github.com> - @@VERSION@@-1
- See https://github.com/jdevera/git-this-bread/releases for release notes
