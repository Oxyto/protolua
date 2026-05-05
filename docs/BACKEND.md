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
- les ports data/reference/impulse normalises;
- les wires entre inputs, locals, `output`, `return`, references de fields et enchainements d'impulses;
- les sorties nommees;
- les diagnostics/warnings backend.

### `brson`

Conteneur binaire experimental:

```sh
protolua compile examples/flux_component.plua -format brson -o out.brson
```

Etat actuel:

- header public `FrDT`;
- 4 octets reserves;
- type d'archive en varbyte;
- `archiveType = 1`, correspondant au mode LZ4 frame documente;
- frame LZ4 valide avec blocs non compresses, pour garder un binaire Go sans dependance externe;
- `inspect-brson` sait aussi lire les blocs LZ4 compresses, pour comparer de futures fixtures exportees par Resonite;
- document BSON racine avec `VersionNumber`, `FeatureFlags`, `Types`, `TypeVersions`, `Object`, `Asset`;
- `Types` contient les noms canoniques CLI-style, et `IComponent.Type` est encode comme index dans cette table;
- `Slot.Components` est encode comme `UniquePtr(Array(IComponent))`;
- les enfants de slots portent un `ParentReference` vers l'ID de leur parent;
- le graph ProtoFlux est materialise en slots debug avec composants `ProtoLua.Runtime.*`, ports et wires normalises;
- les entrees de table d'un `pf.node("...", { ... })` deviennent des ports de node (`Variable`, `Value`, etc.);
- sous-document `ProtoLua` pour garder le record debug complet.

Important: le conteneur utilise maintenant le mode LZ4 frame documente et porte un graphe connecte normalise, mais le fichier n'est pas encore garanti importable par Resonite tant que le layout exact des composants persistants et des references ProtoFlux n'est pas valide.

Inspection:

```sh
protolua inspect-brson out.brson
```

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
3. mapper `UniquePtr`, `WorldElementRef`, `Sync<T>` et les index de types exactement;
4. encoder le binaire `.brson` selon le layout Resonite exact;
5. tester l'import dans Resonite;
6. garder `record` comme format debug stable.
