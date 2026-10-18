Licence texts that a Go module in the image does not carry itself. The build
copies a module's own licence files into `/licenses/<module path>/`, and uses
the file here only for a module whose source has none.

| File | Module | Licence |
|---|---|---|
| `github.com/leodido/go-urn/LICENSE` | github.com/leodido/go-urn v1.1.0 (required by gopkg.in/go-playground/validator.v9 v9.30.0, which ocpp-go v0.19.0 requires). The v1.1.0 module has no licence file; the project added this MIT licence with tag v1.2.0 (commit a0f5013415294bb94553821ace21a1a74c0298cc), and the text is taken from there | MIT |
