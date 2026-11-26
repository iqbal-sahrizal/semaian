# Coffee Knowledge Domain Discovery

## 1. Bounded Contexts & Subdomains
`knowledge` owns canonical coffee reference data and educational editorial content.

### 1.1 Geography & Origin
- **Hierarchy**: `Country` → `Region` → `Subregion` (self-referencing `parent_id` on `regions`) → `Producer/Farm/Cooperative/Washing Station` (optional).
- **Incomplete Data**: Not every coffee requires a farm-level identity; region-level or country-level origin is valid. Altitude (`min_masl`, `max_masl`) and climate notes are optional.

### 1.2 Species & Varieties
- **Species**: e.g., *Coffea arabica*, *Coffea canephora* (Robusta), *Coffea liberica*.
- **Varieties/Cultivars**: Scientific/botanical classification separated from commercial names and synonyms; optional `parentage` notes.

### 1.3 Harvest & Green Coffee (Boundary Decision)
- **General knowledge** (e.g., typical harvest seasons per region, cherry maturity concepts) belongs in `knowledge`.
- **Specific lot/batch data** (lot ID, moisture %, density, defect count, screen size) is **deferred** to future `catalog` / `b2b` modules.

### 1.4 Processing Methods
- Extensible entities (`Washed`, `Natural`, `Honey`, `Anaerobic`, `Wet-Hulled`, `Carbonic Maceration`, hybrid/experimental).
- Processing method alone does not guarantee a specific flavor; structured characteristics are descriptive, not prescriptive.

### 1.5 Roasting
- `knowledge` owns general **Roast Levels** (Light, Medium-Light, Medium, Medium-Dark, Dark) and educational concepts (first crack, second crack, development time).
- Actual roaster-specific roast batch profiles are **deferred** to future `roastery` / `catalog` capabilities.

### 1.6 Sensory & Flavor Taxonomy
- Hierarchical, extensible **Flavor Descriptors** (`family` → `category` → `descriptor`) with multilingual names and synonyms—never a hardcoded enum.
- Sensory attributes (acidity, sweetness, body, bitterness, aftertaste, balance, cleanliness, mouthfeel) are structured reference concepts, not objective guarantees.

### 1.7 Brewing Knowledge vs. Brew Studio Boundary
- **`knowledge` owns**: Reference concepts (brewing methods like V60/Espresso/Tubruk, grinder concepts, water chemistry, ratio/extraction theory).
- **`brew` (Brew Studio, future) owns**: User/org personal recipes, brew logs, timers, and equipment inventories.
