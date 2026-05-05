# Backend ProtoLua

Le backend transforme l'AST ProtoLua en IR, puis en modele de record Resonite/ProtoFlux.

## Formats

### `ir`

Sortie JSON de l'IR frontend:

```sh
protolua compile examples/flux_component.plua -format ir
```

### `record`

Modele de record lisible et deterministe:

```sh
protolua compile examples/flux_component.plua -format record -o out.record.json
```

Le record contient:

- un slot racine;
- un composant racine de graph ProtoFlux;
- les entry points `on`;
- les fonctions;
- les nodes ProtoFlux et leurs inputs;
- les sorties nommees;
- les diagnostics/warnings backend.

### `brson`

Conteneur binaire experimental:

```sh
protolua compile examples/flux_component.plua -format brson -o out.brson
```

Important: ce fichier a l'extension `.brson`, mais l'encodage binaire officiel Resonite Record n'est pas encore implemente. Le conteneur actuel est volontairement marque `protolua.experimental-brson` et emballe le record ProtoLua compresse avec checksum. Le point d'integration est `internal/backend.WriteExperimentalBRSON`.

## Resolution automatique

`-format auto` choisit d'apres l'extension:

- `.brson` -> `brson`
- `.record.json` -> `record`
- autre ou stdout -> `ir`

## Backend sans dependances

Le backend utilise uniquement la bibliotheque standard Go. Il garde donc l'objectif de livraison en un seul binaire par plateforme.

## Prochaine etape pour l'import Resonite

Pour obtenir un `.brson` importable par Resonite, il faut remplacer le writer experimental par un serializer Resonite Record officiel:

1. mapper `backend.Record` vers les types exacts de record Resonite;
2. ecrire les composants et fields avec leurs IDs/types Resonite exacts;
3. encoder le binaire `.brson` selon le layout Resonite;
4. garder `record` comme format debug stable.
