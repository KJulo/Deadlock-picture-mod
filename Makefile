PY := .venv/bin/python

.PHONY: help setup fetch golden test test-go test-py vpk

help:
	@echo "make setup    crea .venv con las dependencias del Python de referencia"
	@echo "make fetch    extrae los archivos base desde tu Deadlock a templates/"
	@echo "make golden   regenera testdata/ (lo que el Go compara contra el Python)"
	@echo "make test     corre los tests de Python y de Go"
	@echo "make test-go  solo Go (ej: make test-go RUN=TestParse)"
	@echo "make vpk PHOTO=assets/cat.jpg [ARGS='-layout wrap -downloads']  arma el VPK con el backend Go"

setup:
	python3 -m venv .venv && .venv/bin/pip install -r reference/requirements.txt

fetch:
	$(PY) reference/scripts/catmod_cli.py fetch

golden:
	$(PY) reference/scripts/make_golden.py

test: test-py test-go

test-py:
	cd reference && ../$(PY) -m pytest -q

test-go:
	cd backend && go test ./... $(if $(RUN),-run $(RUN) -v)

PHOTO ?= assets/cat.jpg
vpk:
	cd backend && go run ./cmd/catmod $(ARGS) ../$(PHOTO)
