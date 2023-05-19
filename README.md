# OSDE2E Framework

This is a proof of concept repository for a common package that can be consumed
by any sort of go program. The primary usage of this package is the following:

* OSDE2E
* OSD SREP Operators

The repository currently contains packages that handle:

* Working with the Managed OpenShift provider (e.g. OSD, ROSA) to
    create/delete clusters
* Different clients (e.g. kubernetes, ocm, prometheus) that can easily
    be consumed

```shell
pkg/
├── clients
│   ├── kubernetes
│   ├── ocm
│   └── prometheus
└── providers
    ├── clouds
    ├── osd
    └── rosa
```
