# ProtoLua

_A Drop-in replacement of ProtoGraph_

## Définition

ProtoLua est une syntaxe inspiré de Luau et de Lua 5.4 afin de remplacer le langage fonctionnel du ProtoGraph. ProtoGraph est un compilateur permettent de convertir du code FSharp en ProtoFlux sur Resonite en le compilant sur le format .brson.

## Features du langage

- Une syntaxe Luau + Lua 5.4
- Des variables ainsi que des fonctions. (local, function)
- Des boucles et des conditions.
- Des maths et des primitives.

## Compilateur

Pour éviter de dépendre du flux-sdk déjà proposé par ProtoGraph, celui-ci utilisera son propre compilateur en 1 seul fichier binaire et il sera écrit en Go par simplicité de développement.