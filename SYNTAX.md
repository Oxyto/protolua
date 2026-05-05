# Syntaxe ProtoLua

ProtoLua est une syntaxe imperative inspiree de Lua 5.4 et Luau, avec un espace de noms `pf` pour exprimer les interactions ProtoFlux, Slots, Components, fields, drivers et dynamic variables.

Le but est de garder le code proche de Lua tout en produisant un IR facile a abaisser en graph ProtoFlux puis en `.brson`.

## Principes

- Le flux de controle est textuel et imperative: `if`, `while`, `for`, `function`, `return`.
- Les valeurs suivent les primitives Lua: nombres, strings, booleens et `nil`.
- Les annotations de type sont optionnelles et servent au backend ProtoFlux: `Slot`, `Component`, `float3`, `color`, `FrooxEngine.UIX.Text`.
- Les operations ProtoFlux explicites passent par `pf.*`.
- Les champs de composants peuvent etre lus comme sources, ecrits une fois, drives en continu, ou references.
- Un appel `pf.*` inconnu reste representable par l'IR generique `ProtoFluxIntrinsic`, ce qui permet d'utiliser un noeud ProtoFlux pas encore specialise dans le compilateur.

## Programme

Un fichier `.plua` est une suite de statements:

```lua
local root: Slot = pf.root()

function clamp01(value: float): float
  if value < 0 then
    return 0
  elseif value > 1 then
    return 1
  end
  return value
end
```

La forme `-> (...)` declare des sorties nommees. Elle est utile pour les events, les helpers ProtoFlux et les groupes qui exposent plusieurs outputs:

```lua
on compute(a: float, b: float) -> (sum: float, product: float) do
  output sum = a + b
  output product = a * b
end
```

## Types optionnels

Les annotations sont facultatives:

```lua
local slot: Slot = pf.root()
local label: Component = pf.component(slot, "FrooxEngine.UIX.Text")

function move(target: Slot, position: float3)
  write target.Position = position
end
```

Les noms de type acceptent les chemins avec `.`:

```lua
local text: FrooxEngine.UIX.Text = pf.component(ui, "FrooxEngine.UIX.Text")
```

## Variables

```lua
local x = 10
local y: float = x * 2
x = x + 1
```

Une affectation sur un identifiant devient une variable locale ProtoLua. Une affectation sur un membre devient une ecriture ProtoFlux:

```lua
local text = pf.component(ui, "FrooxEngine.UIX.Text")
text.Content = "Ready"       -- ecriture ProtoFlux one-shot
write text.Content = "Ready" -- forme explicite equivalente
```

## Fonctions

```lua
function distance(a: float3, b: float3): float
  return pf.node("Operators.Distance", { A = a, B = b })
end
```

Sortie unique avec `->`:

```lua
function is_visible(slot: Slot) -> bool
  return pf.source(slot.ActiveSelf)
end
```

Sorties multiples:

```lua
function divmod(value: int, divisor: int) -> (quotient: int, remainder: int)
  return value / divisor, value % divisor
end
```

Les fonctions sont abaissees en blocs IR `Function`. Le backend pourra les materialiser comme groupes de noeuds, locals et continuations ProtoFlux.

## Conditions

```lua
if enabled then
  write indicator.Enabled = true
elseif debugMode then
  pf.debug_log("indicator disabled")
else
  write indicator.Enabled = false
end
```

## Boucles

Boucle `while`:

```lua
while count < 10 do
  count = count + 1
end
```

Boucle numerique:

```lua
for i = 1, 8 do
  pf.debug_log(i)
end

for i = 10, 1, -1 do
  pf.debug_log(i)
end
```

## Evenements et impulses

`on` decrit un point d'entree ProtoFlux:

```lua
on start do
  pf.debug_log("started")
end

on update(deltaTime: float, frameIndex: int) do
  drive spinner.Rotation = quat(0, deltaTime, 0, 1)
end
```

Les inputs multiples sont declares comme les parametres d'une fonction. Les outputs multiples utilisent `-> (...)` et sont alimentes par `output`:

```lua
on evaluate(value: float, threshold: float) -> (passed: bool, delta: float) do
  output passed = value >= threshold
  output delta = value - threshold
end
```

Le nom d'evenement peut etre une string si le backend doit garder un nom Resonite exact:

```lua
on "OnUserJoined" do
  pf.debug_log("user joined")
end
```

## Expressions

Primitives:

```lua
nil
true
false
123
3.14
"text"
```

Operateurs:

```lua
a + b
a - b
a * b
a / b
a % b
a ^ b
a .. b
a == b
a ~= b
a < b
a <= b
a > b
a >= b
not enabled
```

Constructeurs reconnus par l'IR:

```lua
float2(1, 2)
float3(0, 1, 0)
float4(0, 1, 0, 1)
color(0.1, 0.8, 1.0, 1.0)
quat(0, 0, 0, 1)
type("FrooxEngine.Slot")
```

Tables Lua simples pour les options et les entrees nommees:

```lua
{ direct = true, nonPersistent = false }
{ X = 1, Y = 2, Z = 3 }
```

## Slots

```lua
local root = pf.root()
local self = pf.this()
local ui = pf.find_slot(root, "UI")
local labelSlot = pf.child(ui, "Label")
local parent = pf.parent(labelSlot)
local children = pf.children(ui)
```

Creation et destruction:

```lua
local created = pf.create_slot(root, "RuntimeLabel", { persistent = false })
pf.set_active(created, true)
pf.destroy(created)
```

`pf.slot(path)` represente une reference de slot connue par chemin ou identifiant projet:

```lua
local avatarRoot = pf.slot("/User/Avatar")
```

## Components

Lecture ou recherche:

```lua
local text = pf.component(labelSlot, "FrooxEngine.UIX.Text")
local renderers = pf.components(root, "FrooxEngine.UIX.Text")
local ownerSlot = pf.get_slot(text)
```

Ajout et suppression:

```lua
local text = pf.add_component(labelSlot, "FrooxEngine.UIX.Text", {
  persistent = true,
})

pf.remove_component(text)
```

Activation:

```lua
local enabled = pf.enabled(text)
pf.set_enabled(text, true)
```

## Fields: source, write, drive, reference

ProtoFlux expose trois manieres classiques d'interagir avec un champ de Slot ou Component:

```lua
local content = pf.source(text.Content)     -- source lisible
local contentRef = pf.ref(text.Content)     -- reference de field
write text.Content = "Ready"                -- ecriture ponctuelle
drive text.Color = color(0.1, 0.8, 1, 1)   -- drive continu
```

Formes fonctionnelles equivalentes:

```lua
pf.write(text.Content, "Ready")
pf.drive(text.Color, color(1, 1, 1, 1))
pf.reference(text.Content)
pf.field(text, "Content")
pf.get(text.Content)
pf.ref_to_output(pf.ref(text.Content))
```

Regle pratique:

- `write` sert aux modifications one-shot, comme le node ProtoFlux Write.
- `drive` sert aux valeurs maintenues chaque frame.
- `pf.source` lit une valeur de field.
- `pf.ref` donne une reference de field pour les nodes qui attendent une reference.
- `pf.ref_to_output` convertit une reference ProtoFlux en variable utilisable par les writes indirects.

Listes de fields:

```lua
local items = pf.field_list(component, "Items")
local count = pf.list.count(items)
local first = pf.list.get(items, 0)

pf.list.set(items, 0, first)
pf.list.add(items, newItem)
pf.list.insert(items, 1, newItem)
pf.list.remove(items, 0)
pf.list.clear(items)
```

## Dynamic variables

Espaces:

```lua
pf.dyn.space(root, "ProtoLua")
```

Lecture:

```lua
local title = pf.dyn.read(root, "ProtoLua.Title", "string")
local sameSpaceTitle = pf.dyn.input("ProtoLua.Title", "string")
local titleWithEvents = pf.dyn.input_events("ProtoLua.Title", "string")
```

Ecriture:

```lua
pf.dyn.write(root, "ProtoLua.Title", "Ready")
```

Creation:

```lua
pf.dyn.create(root, "ProtoLua.Count", 0, {
  direct = true,
  nonPersistent = false,
})
```

Write-or-create:

```lua
pf.dyn.write_or_create(root, "ProtoLua.Title", "Ready", {
  direct = true,
  nonPersistent = false,
})
```

Suppression et nettoyage:

```lua
pf.dyn.delete(root, "ProtoLua.Title", "string")
pf.dyn.clear(root)
pf.dyn.clear_type(root, "string")
```

Driver de dynvar vers field:

```lua
pf.dyn.drive(root, "ProtoLua.Title", text.Content)
```

## Noeuds ProtoFlux generiques

Pour couvrir un node non encore expose par une fonction dediee:

```lua
local value = pf.node("Operators.ValueAdd<float>", {
  A = 1,
  B = 2,
})

pf.node("Actions.Write", {
  Variable = pf.ref(text.Content),
  Value = "Hello",
})
```

Appels d'impulse:

```lua
pf.impulse(writer, "Write")
pf.delay(0.25)
pf.debug_log("done")
```

Packing:

```lua
pf.pack(root, { name = "ProtoLuaGraph" })
pf.unpack(root)
```

## Inputs et outputs

Les entrees de fonctions et d'events sont des parametres:

```lua
on update(deltaTime: float, frameIndex: int, user: User) do
  pf.debug_log(deltaTime)
end
```

Les sorties se declarent avec `->`:

```lua
on raycast(origin: float3, direction: float3) -> (hit: bool, point: float3) do
  local result = pf.node("Physics.Raycast", {
    Origin = origin,
    Direction = direction,
  })

  output hit = result.Hit
  output point = result.Point
end
```

Dans une fonction, `return` peut renvoyer plusieurs valeurs:

```lua
function minmax(a: float, b: float) -> (min: float, max: float)
  if a < b then
    return a, b
  end
  return b, a
end
```

## Exemple complet

```lua
on start do
  local root: Slot = pf.root()
  local ui: Slot = pf.find_slot(root, "UI")
  local text: Component = pf.component(ui, "FrooxEngine.UIX.Text")

  write text.Content = "ProtoLua ready"
  drive text.Color = color(0.2, 0.8, 1.0, 1.0)

  pf.dyn.space(root, "ProtoLua")
  pf.dyn.write_or_create(root, "ProtoLua.Status", "Ready", {
    direct = true,
    nonPersistent = false,
  })
end
```

## Compatibilite ProtoFlux

La syntaxe suit les concepts publics de ProtoFlux:

- Les champs de Slot/Component peuvent produire des nodes Source, Drive ou Reference.
- Le node Write modifie une variable/field ponctuellement sans drive continu.
- Les dynamic variables se manipulent via read, write, create et write-or-create.
- L'IR garde une operation generique `ProtoFluxIntrinsic` pour les nodes ProtoFlux qui ne sont pas encore specialises.

References utiles:

- https://wiki.resonite.com/ProtoFlux
- https://wiki.resonite.com/ProtoFlux%3AWrite
- https://wiki.resonite.com/ProtoFlux%3AReadDynamicVariable
- https://wiki.resonite.com/ProtoFlux%3AWriteDynamicVariable
- https://wiki.resonite.com/ProtoFlux%3ACreateDynamicVariable
- https://wiki.resonite.com/ProtoFlux%3AWriteOrCreateDynamicVariable
- https://wiki.resonite.com/Dynamic_variables/fr
