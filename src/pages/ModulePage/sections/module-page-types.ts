import type { LucideIcon } from "lucide-react";

export type ModuleCatalogItem = {
  id: string;
  label: string;
  description: string;
  icon: LucideIcon;
  aliases: string[];
};

export type ModuleStatusCard = ModuleCatalogItem & {
  cardID: string;
  moduleKey: string;
  installed: boolean;
  runtimeStatus: string;
  endpoint: string;
  sourceName: string;
};

export type BoardView = "all" | "installed" | "pending";
