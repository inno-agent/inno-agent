import { useContext } from "react";
import { SettingsContext } from "./settingsContext";

export function useSettings() {
    return useContext(SettingsContext);
}
