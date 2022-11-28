import View from "./View";
import React, { useState } from "react";
import { AllViewsResponse, ViewInterface } from "./types";
import { useGetTableViewsQuery } from "./viewsApiSlice";
import { ColumnMetaData } from "../table/types";

export function useView<T>(
  page: keyof AllViewsResponse,
  allColumns: Array<ColumnMetaData<T>>,
  defaultView: number,
  viewTemplate: ViewInterface<T>
): [View<T>, React.Dispatch<React.SetStateAction<number>>, boolean] {
  const allViews = useGetTableViewsQuery<{
    data: AllViewsResponse;
    isLoading: boolean;
    isFetching: boolean;
    isUninitialized: boolean;
    isSuccess: boolean;
  }>();
  const invoiceViews = allViews?.data ? (allViews.data[page] as Array<typeof viewTemplate>) : [viewTemplate];

  const [updateCounter, viewUpdater] = useState(0);
  const [selectedView, setSelectedView] = useState(defaultView);
  const view = new View(invoiceViews[selectedView], allColumns, updateCounter, viewUpdater);

  return [view, setSelectedView, allViews.isSuccess];
}
