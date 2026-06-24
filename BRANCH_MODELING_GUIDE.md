# Guía de Branch Modeling para repositorios Paceron

## Requisitos previos

- Tener `gh` (GitHub CLI) instalado: `brew install gh`
- Haber iniciado sesión con un token classic con scope `repo`:
  ```bash
  echo "ghp_tu_token" | gh auth login --with-token
  ```
- Tener permisos de **admin** en el repositorio destino

---

## 1. Hacer el repositorio público (si es privado)

GitHub Free no permite proteger ramas en repos privados.

```bash
gh api repos/:owner/:repo -f private=false
```

> Alternativa: hacerlo desde GitHub.com → Settings → Danger Zone → Change visibility.

---

## 2. Crear rama `master` desde `main` (si existe)

```bash
# Crear master local desde main remota
git fetch origin main
git branch master origin/main
git push origin master

# Cambiar default branch del repo
gh api repos/:owner/:repo -f default_branch=master

# Eliminar main remota
gh api repos/:owner/:repo/git/refs/heads/main -X DELETE
```

---

## 3. Crear rama `develop`

```bash
# Obtener SHA del último commit en master
SHA=$(gh api repos/:owner/:repo/git/refs/heads/master --jq '.object.sha')

# Crear develop desde master
gh api repos/:owner/:repo/git/refs -f ref=refs/heads/develop -f sha=$SHA
```

---

## 4. Proteger ramas `master` y `develop`

```bash
# Proteger master
gh api repos/:owner/:repo/branches/master/protection -X PUT \
  --input - <<'EOF'
{
  "required_status_checks": null,
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true
  },
  "restrictions": null,
  "required_linear_history": false,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
EOF

# Proteger develop (mismos parámetros)
gh api repos/:owner/:repo/branches/develop/protection -X PUT \
  --input - <<'EOF'
{
  "required_status_checks": null,
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true
  },
  "restrictions": null,
  "required_linear_history": false,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
EOF
```

---

## 5. Crear rulesets por patrón de rama

Protege los patrones de ramas de soporte contra force push y eliminación:

```bash
for branch in "feature/*" "release/*" "hotfix/*" "fix/*" "backport/*"; do
  name="${branch%%/*}"
  gh api repos/:owner/:repo/rulesets -X POST --input - <<EOF
  {
    "name": "${name}-rules",
    "target": "branch",
    "enforcement": "active",
    "conditions": {
      "ref_name": {
        "include": ["refs/heads/${branch}"],
        "exclude": []
      }
    },
    "rules": [
      { "type": "deletion" },
      { "type": "non_fast_forward" }
    ]
  }
EOF
done
```

> Opcional: para mayor rigor, agregar el rule type `"pull_request"` con `required_approving_review_count`.

---

## 6. Agregar PR Template

Crear `.github/PULL_REQUEST_TEMPLATE.md`:

```markdown
## Título

[Tipo] Ticket: Descripción corta

---

## Descripción

<!-- Qué hace, por qué se hace, impacto -->

---

## Contexto / Background

<!-- Enlace a ticket, decisiones técnicas, alternativas descartadas -->

---

## Cómo probarlo

<!-- Pasos claros, credenciales o links, URLs, mocks -->
1.
2.
3.

---

## Screenshots / GIFs

<!-- Obligatorio si hay UI, logs o flujos visuales -->

---

## Checklist de Auto-Verificación

- [ ] Tests pasan localmente
- [ ] CI verde
- [ ] Sin console.log de debug

---

## Tickets / Issues

<!-- Closes #N, Relates to JIRA-XXX -->
```

---

## 7. Actualizar repo local

```bash
git fetch origin
git branch -u origin/master master
git checkout -b develop origin/develop
git checkout master
```

---

## Flujo de ramas establecido

| Tipo de rama          | Origen    | Destino   | Propósito                              |
|-----------------------|-----------|-----------|----------------------------------------|
| `feature/<nombre>`    | develop   | develop   | Nueva funcionalidad                    |
| `release/<versión>`   | develop   | master    | Preparación de versión                 |
| `fix/<id>`            | release/ o develop | release/ o develop | Corrección en release o develop |
| `hotfix/<id>`         | master    | master    | Corrección urgente en producción       |
| `backport/<versión>`  | master    | develop   | Sincronizar cambios de producción      |

### Reglas clave

- **Nadie hace push directo a `master` ni `develop`** — todo pasa por Pull Request con al menos 1 aprobación.
- Las `feature/*` se crean desde `develop` y se mergean vía PR a `develop`.
- Las `release/*` se crean desde `develop` y se mergean vía PR a `master`.
- Las `hotfix/*` se crean desde `master` y se mergean vía PR a `master`.
- Después de un release, se crea `backport/<versión>` desde `master` y se mergea a `develop` para mantener sincronización.
