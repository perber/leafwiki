import { useContext } from "react";
import { PageToolbarContext } from "./PageToolbarContext";

export function usePageToolbar() {
    return useContext(PageToolbarContext)
}
