# TODO ProtoLua

Fichier de suivi des choses restantes avant un compilateur ProtoLua utilisable en production avec Resonite.

## Priorite 1: `.brson` Resonite importable

- [x] Construire un modele backend `record` stable depuis l'IR ProtoLua.
- [x] Remplacer le conteneur experimental `PLBRSON` par un writer avec header Resonite `FrDT`.
- [x] Ajouter un encodeur BSON interne sans dependance externe.
- [x] Emettre un document racine proche du schema public: `VersionNumber`, `FeatureFlags`, `Types`, `TypeVersions`, `Object`, `Asset`.
- [x] Remplacer l'archive raw BSON par une archive LZ4 frame (`archiveType = 1`) sans dependance externe.
- [x] Valider le type d'archive documente: `0x01 = LZ4 frame mode`.
- [x] Lire les blocs LZ4 compresses dans `inspect-brson` pour futures fixtures exportees.
- [ ] Valider l'archive LZ4 produite directement par import Resonite.
- [x] Encoder `Components` comme `UniquePtr(Array(IComponent))`.
- [x] Encoder `ParentReference` de Slot comme `PackageNodeIdentifier`.
- [x] Ajouter des helpers backend pour `UniquePtr`, `WorldElementRef`, `Sync<T>` et `SyncList<T>`.
- [ ] Valider `WorldElementRef` contre une fixture Resonite reelle.
- [ ] Mapper tous les `SyncRef<T>`/`SyncList<T>` reels de components depuis des fixtures.
- [x] Renseigner les noms de types canoniques emis dans `Types`, par exemple `[FrooxEngine]FrooxEngine.Slot`.
- [x] Encoder `IComponent.Type` comme `ComponentTableIndex`.
- [x] Materialiser le graphe ProtoFlux en slots/components debug avec ports, wires data/reference et wires d'impulse normalises.
- [x] Exposer les entrees de tables `pf.node("...", { ... })` comme ports de node dedies.
- [ ] Mapper les composants ProtoFlux reels avec leurs champs persistants exacts.
- [ ] Generer les references de ports, wires, impulses et drives selon le layout ProtoFlux reel.
- [ ] Importer un fichier produit dans Resonite et corriger jusqu'a validation.
- [ ] Ajouter des fixtures `.brson` exportees depuis Resonite pour tests de compatibilite.

## Backend ProtoFlux

- [ ] Construire un catalogue complet des nodes ProtoFlux connus: chemin, inputs, outputs, types generiques, impulses et defaults.
- [ ] Mapper les signatures ProtoFlux generiques (`T`, `IWorldElement`, `SyncRef<T>`, collections, refs) vers le systeme de types ProtoLua.
- [ ] Ajouter un resolver de surcharge pour choisir le bon node ProtoFlux selon les types des arguments.
- [ ] Resoudre les types `FrooxEngine.*` et les generics ProtoFlux.
- [ ] Valider les fields de components (`Content`, `Color`, `Enabled`, etc.).
- [ ] Transformer `if`, `while`, `for`, `function`, `on`, `return`, `output` en vrais nodes ProtoFlux Resonite connectes.
- [ ] Supporter les continuations/impulses de controle avec branches, merges, delays, async et sequence explicite.
- [x] Generer un graphe normalise avec ports d'inputs/outputs, `return`, `output`, wires de dependance et enchainement d'impulses.
- [ ] Placer les nodes et groupes de facon lisible dans le graph.
- [ ] Materialiser `write`, `drive`, `pf.source`, `pf.ref`, dynvars et field lists avec les nodes Resonite exacts.
- [ ] Materialiser les events Resonite courants (`Start`, `Update`, user events, interactions, collisions, equip, UIX) avec leurs inputs exacts.
- [ ] Supporter les appels entre fonctions ProtoLua comme groupes/subgraphs ProtoFlux reutilisables.
- [ ] Supporter les nodes qui exposent plusieurs outputs data et plusieurs impulses.
- [ ] Supporter les conversions automatiques utiles pour garder une syntaxe simple: refs, sources, values, casts numeriques et generics inferes.
- [ ] Ajouter un mode strict qui refuse tout node/field non resolu, et un mode permissif qui garde `pf.node`/`ProtoFluxIntrinsic`.
- [ ] Ajouter une passe d'optimisation simple: constantes, dead nodes, locals inutilises.

## Analyse semantique

- [x] Variables non declarees.
- [x] Variables redefinies dans le meme scope.
- [x] Mauvais nombre d'arguments sur `pf.*` pour les helpers connus.
- [x] Outputs declares mais jamais assignes.
- [x] `output` vers un nom non declare.
- [x] Incompatibilites de types simples.
- [x] Inference de types locale pour eviter d'ecrire des annotations partout.
- [ ] Verification des fields de components selon le type reel du component.
- [ ] Verification des tables d'options `pf.*` et des ports nommes de `pf.node`.
- [ ] Verification des retours multiples et outputs multiples dans tous les chemins de controle.
- [ ] Diagnostics avec ranges source precises.

## Langage

- [ ] Definir officiellement le sous-ensemble Lua vise: ce qui doit rester identique a Lua standard, ce qui est volontairement ProtoLua.
- [ ] Garder les scripts simples lisibles comme du Lua: moins de `pf.*` obligatoire quand le type permet d'inferer l'operation ProtoFlux.
- [x] Ajouter des sucres syntaxiques Lua-like pour les cas ProtoFlux frequents: field source/ref/write/drive, component lookup, slot lookup, dynvars.
- [x] Definir deux profils de syntaxe: `lua-compatible` avec uniquement des formes valides Lua, et `protolua-extended` avec les sucres `on`, `output`, `write ... =`, `drive ... =`.
- [x] Ajouter une prelude Lua-compatible qui alias les operations ProtoFlux frequentes sous forme d'appels Lua: `root()`, `this()`, `slot(path)`, `node(path, inputs)`, `source(field)`, `ref(field)`, `write(field, value)`, `drive(field, value)`.
- [x] Garder `pf.*` comme escape hatch bas niveau, mais documenter les alias simples comme API principale.
- [x] Ajouter une syntaxe method-call Lua pour les slots/components: `root:find("UI")`, `slot:child("Label")`, `slot:parent()`, `slot:children()`, `slot:component("Type")`, `slot:add_component("Type", opts)`.
- [x] Ajouter une syntaxe method-call Lua pour les fields/dynvars: `field:source()`, `field:ref()`, `dyn("Space.Name"):read("string")`, `dyn("Space.Name"):write(value)`, `dyn("Space.Name"):drive(field)`.
- [x] Ajouter une alternative Lua-compatible a `on`: table d'events ou declaration standard Lua, par exemple `events.start = function() ... end` et `events.update = function(deltaTime) ... end`.
- [x] Ajouter une alternative Lua-compatible a `output`: `return { passed = ..., delta = ... }` pour outputs nommes, et `return a, b` pour outputs positionnels.
- [ ] Ajouter une alternative Lua-compatible aux annotations `: Type`: commentaires type-style `---@param`, `---@return`, `---@type`, tout en gardant les annotations courtes du mode etendu.
- [ ] Formaliser les regles d'inference pour eviter `pf.source`/`pf.ref`: lecture de field en expression -> source, field passe a un port reference -> ref, affectation de field -> write ponctuel.
- [x] Garder `drive(...)` explicite pour les valeurs continues afin d'eviter qu'une affectation Lua standard change de semantique selon le contexte.
- [x] Ajouter une commande de verification `check --profile lua-compatible` qui refuse les sucres non Lua standard.
- [x] `repeat ... until`.
- [x] `break` et `continue`.
- [x] Tables plus completes: index numeriques, champs imbriques, champs implicites, trailing separators partout.
- [x] Supporter les commentaires Lua longs `--[[ ... ]]`.
- [x] Supporter les fonctions locales `local function name(...) ... end`.
- [x] Supporter les appels et affectations multiples Lua-like (`local a, b = ...`, `a, b = b, a`).
- [x] Supporter les varargs `...` si utiles pour helpers et wrappers.
- [x] Method calls `slot:method()` avec lowering semantique.
- [ ] Modules/imports si plusieurs fichiers deviennent necessaires.
- [ ] Ajouter une petite stdlib compatible Lua quand elle se compile bien vers ProtoFlux: `math`, `string`, `table` utiles.
- [ ] Decider explicitement si metatables, coroutines, IO/filesystem et debug Lua sont exclus, simules ou partiellement supportes.
- [ ] Ajouter une suite d'exemples "Lua simple vers ProtoFlux" pour mesurer la simplicite de la syntaxe.

## LSP

- [x] Diagnostics syntaxiques.
- [x] Completion mots-cles/types/`pf.*`/aliases Lua-compatible.
- [x] Semantic tokens.
- [x] Diagnostics semantiques de base.
- [x] Signature help pour `pf.*`.
- [ ] Go-to-definition.
- [ ] Rename symbol.
- [ ] Format document.

## Editeurs

- [x] Extension VSCode locale.
- [x] Extension Zed locale.
- [ ] Packager et tester le `.vsix` VSCode.
- [ ] Tester l'extension Zed dans l'editeur.
- [ ] Publier instructions d'installation finales.

## Distribution

- [x] Builds `CGO_ENABLED=0` Linux/macOS/Windows.
- [x] Checksums de release.
- [x] CI de tests.
- [x] CI de cross-compilation.
