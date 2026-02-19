# snip — CLI Token Killer

> Un proxy CLI open source qui réduit la consommation de tokens LLM de 60 à 90 %, écrit en Go, extensible par la communauté via un registry de filtres décentralisé.

---

## Le problème

Les agents de code comme Claude Code, Cursor ou Aider consomment des tokens proportionnellement à la verbosité des outputs shell — pas à leur utilité réelle. Un `git push` génère quinze lignes pour transmettre une seule information : succès ou échec. Un `go test` qui passe produit des centaines de lignes de logs que le LLM n'utilisera jamais.

Le résultat : des sessions qui s'épuisent prématurément, des coûts API injustifiés, et un agent qui se noie dans du bruit au lieu de raisonner sur du signal.

## La solution existante et ses limites

rtk (Rust Token Killer) a démontré la valeur du concept : intercepter les outputs shell avant qu'ils atteignent le contexte LLM, filtrer le bruit, reformater l'essentiel. Les économies mesurées atteignent 70 à 90 % selon les commandes.

Mais rtk souffre d'une fragilité structurelle : chaque filtre est du code Rust compilé dans le binaire. Quand un outil externe change son format d'output, le filtre casse. La correction exige une PR sur le dépôt principal, une review, une release, une réinstallation. Le cycle est lent, la dépendance à une équipe centrale est totale.

C'est un outil brillant avec une architecture qui ne peut pas passer à l'échelle communautaire.

## La vision de snip

snip reprend le concept de rtk en Go et y ajoute une couche fondamentale : un registry décentralisé de filtres, maintenable par la communauté sans intervention sur le binaire.

Le binaire snip est le moteur. Les filtres sont les données. Les deux évoluent indépendamment.

---

## Le registry de filtres

### Principe

Un filtre est un fichier de configuration déclaratif — pas du code compilé. Il décrit comment traiter l'output d'une commande donnée : quelles lignes supprimer, quelles lignes conserver, comment reformater le résultat, quel template de résumé appliquer.

Ces fichiers sont hébergés dans un dépôt public versionné, séparé du binaire. snip les télécharge et les met en cache localement. À chaque lancement, il vérifie silencieusement si des mises à jour sont disponibles.

### Ce que ça change

N'importe qui peut écrire un filtre pour un outil non supporté et le soumettre au registry. La correction d'un filtre cassé ne nécessite pas de recompiler le binaire. Un développeur isolé sur un stack exotique peut publier et partager son filtre sans dépendre d'une équipe centrale. Les filtres peuvent être versionnés : si une mise à jour casse quelque chose, on revient en arrière en une ligne.

### Gouvernance

Le registry principal est un dépôt GitHub public sous licence MIT. Les filtres soumis passent par une review légère : vérification de format, test sur des outputs réels, validation qu'aucune donnée sensible n'est exfiltrée. Les filtres validés sont signés numériquement. snip refuse d'appliquer un filtre non signé par le registry officiel, sauf opt-in explicite pour les filtres locaux ou tiers.

---

## Architecture du projet

### Deux dépôts distincts

Le premier dépôt contient le binaire snip : le moteur de dispatch, le système de cache des filtres, la couche de mise à jour du registry, le tracking des économies de tokens. C'est le cœur stable du projet, qui évolue lentement.

Le second dépôt est le registry lui-même : une collection de fichiers de filtres organisés par outil, versionnés, signés, ouverts aux contributions externes. C'est le cœur vivant du projet, qui évolue au rythme de la communauté.

### Le hook Claude Code

Comme rtk, snip s'installe comme hook PreToolUse dans Claude Code. Les commandes sont interceptées et réécrites de façon transparente avant exécution. L'agent ne voit que l'output filtré.

L'installation est une commande unique. Le hook est un script shell minimaliste qui appelle snip. Aucune configuration manuelle requise.

---

## Philosophie open source

snip est et restera entièrement open source sous licence MIT, sans exception.

Pas de version pro. Pas de fonctionnalités réservées. Pas de télémétrie sans consentement explicite. Le tracking des économies de tokens est local, stocké dans une base SQLite sur la machine de l'utilisateur, jamais transmis.

La valeur du projet repose sur la communauté qui maintient les filtres. En retour, la communauté a un accès total au code, à l'architecture, et aux décisions de gouvernance.

---

## Pourquoi Go

Go produit des binaires statiques cross-platform sans dépendance externe. La distribution est simple : un seul fichier, pas de runtime, pas d'installation de toolchain pour l'utilisateur final.

La bibliothèque standard Go couvre l'ensemble des besoins : parsing de texte, expressions régulières, HTTP pour le registry, SQLite via un driver pure Go pour le tracking. Pas de dépendances lourdes.

Le code est lisible, maintenable par une communauté large, et compilable par n'importe qui en une commande. C'est cohérent avec une philosophie open source réelle.

### Avantage architectural sur rtk : la concurrence

Un proxy subprocess doit résoudre un problème classique : lire stdout et stderr simultanément. Si les deux streams sont lus séquentiellement, le deadlock est inévitable — le subprocess bloque en attendant que son pipe stderr soit vidé, pendant que le proxy attend que stdout se ferme. Les deux s'attendent mutuellement.

rtk résout ce problème avec deux threads OS, un par stream, faute d'async runtime dans sa configuration actuelle. C'est correct, mais coûteux : chaque commande interceptée alloue deux threads.

Go résout ce problème naturellement avec des goroutines. Deux goroutines légères lisent stdout et stderr en parallèle, communiquent via un channel, et le filtre s'applique à la volée ligne par ligne, sans bufferiser l'intégralité de l'output en mémoire. Le scheduler Go multiplex ces goroutines sur les threads disponibles sans surcoût d'allocation.

Ce n'est pas un argument de performance abstraite. Pour un outil CLI qui s'intercale dans chaque commande d'une session de développement, la latence de démarrage et l'empreinte mémoire par invocation sont des critères concrets. Go est structurellement mieux positionné que Rust sans runtime async pour ce cas d'usage précis.

---

## État du projet

snip est une vision en cours de formalisation. Le développement n'a pas encore commencé. Ce document est le point de départ.

Les contributions à la réflexion, à l'architecture, et à la rédaction des premiers filtres sont bienvenues avant même la première ligne de code.

---

## Contribuer

Le dépôt du registry sera le point d'entrée principal pour la communauté. Écrire un filtre ne nécessite pas de connaître Go. Il suffit de comprendre le format de l'output de l'outil cible et de savoir ce qu'un LLM a besoin d'en retenir.

C'est volontairement accessible : la barrière à l'entrée doit être minimale pour que le registry grandisse vite et reste à jour.
