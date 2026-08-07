## ADDED Requirements

### Requirement: Selección explícita de stage de Supabase
El sistema SHALL conectarse al proyecto de Supabase de testing por default, y SHALL requerir el flag exacto `--stage=production` para conectarse al proyecto de producción.

#### Scenario: Arranque sin flag
- **WHEN** el binario arranca sin argumentos, o con argumentos que no incluyen `--stage=production`
- **THEN** el sistema usa `SUPABASE_TESTING_DATABASE_URL` y loguea `stage=testing` al arrancar

#### Scenario: Arranque con el flag de producción
- **WHEN** el binario arranca con el argumento exacto `--stage=production`
- **THEN** el sistema usa `SUPABASE_PRODUCTION_DATABASE_URL` y loguea `stage=PRODUCTION` al arrancar

#### Scenario: Flags no relacionados presentes
- **WHEN** el binario arranca con otros argumentos (ej. flags de test, typos) que no son exactamente `--stage=production`
- **THEN** el sistema cae en testing stage, no en producción

### Requirement: Deploys de Render separados por stage
El sistema SHALL desplegarse en dos services de Render independientes: uno para `master` contra producción, otro para `develop` contra testing.

#### Scenario: Deploy de master
- **WHEN** se hace push a `master`
- **THEN** el service `paceron-backend` se redespliega con `--stage=production`, contra `SUPABASE_PRODUCTION_DATABASE_URL`

#### Scenario: Deploy de develop
- **WHEN** se hace push a `develop`
- **THEN** el service `paceron-backend-develop` se redespliega sin flag de stage, contra `SUPABASE_TESTING_DATABASE_URL`
