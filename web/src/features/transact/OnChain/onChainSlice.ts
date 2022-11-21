export const a = 1;
// import { createSlice, PayloadAction } from "@reduxjs/toolkit";
// import { ColumnMetaData } from "features/table/types";
// import { RootState } from "store/store";
// import { torqApi } from "apiSlice";
// import { ViewInterface } from "features/viewManagement/types";
// import { OnChainTx } from "./types";
// import { AllOnChainColumns } from "./onChainDefaults";
//
// export interface TableOnChainState {
//   onChainViews: ViewInterface<OnChainTx>[];
//   selectedViewIndex: number;
// }
//
// export const initialState: TableOnChainState = {
//   selectedViewIndex: 0,
//   onChainViews: [
//     {
//       ...DefaultOnChaninView,
//       title: "Default View",
//     },
//   ],
// };
//
// export const onChainSlice = createSlice({
//   name: "onChain",
//   initialState,
//   reducers: {
//     updateOnChainFilters: (state, actions: PayloadAction<{ filters: any }>) => {
//       state.onChainViews[state.selectedViewIndex].filters = actions.payload.filters;
//     },
//     updateColumns: (state, actions: PayloadAction<{ columns: ColumnMetaData<OnChainTx>[] }>) => {
//       state.onChainViews[state.selectedViewIndex].columns = actions.payload.columns;
//     },
//     updateViews: (state, actions: PayloadAction<{ views: ViewInterface<OnChainTx>[]; index: number }>) => {
//       state.onChainViews = actions.payload.views;
//       state.selectedViewIndex = actions.payload.index;
//     },
//     updateViewsOrder: (state, actions: PayloadAction<{ views: ViewInterface<OnChainTx>[]; index: number }>) => {
//       state.onChainViews = actions.payload.views;
//       state.selectedViewIndex = actions.payload.index;
//     },
//     deleteView: (state, actions: PayloadAction<{ view: ViewInterface<OnChainTx>; index: number }>) => {
//       state.onChainViews = [
//         ...state.onChainViews.slice(0, actions.payload.index),
//         ...state.onChainViews.slice(actions.payload.index + 1, state.onChainViews.length),
//       ];
//       state.selectedViewIndex = 0;
//     },
//     updateSelectedView: (state, actions: PayloadAction<{ index: number }>) => {
//       state.selectedViewIndex = actions.payload.index;
//     },
//   },
//   // The `extraReducers` field lets the slice handle actions defined elsewhere,
//   // including actions generated by createAsyncThunk or in other slices.
//   extraReducers: (builder) => {
//     builder.addMatcher(
//       (action) => {
//         return (
//           ["onChain/updateOnChainFilters", "onChain/updateColumns"].findIndex((item) => action.type === item) !== -1
//         );
//       },
//       (state, _) => {
//         state.onChainViews[state.selectedViewIndex].saved = false;
//       }
//     );
//
//     builder.addMatcher(torqApi.endpoints.createTableView.matchFulfilled, (state, { payload }) => {
//       state.onChainViews[payload.index] = {
//         ...payload.view.view,
//         id: payload.view.id,
//       };
//       state.selectedViewIndex = payload.index;
//     });
//
//     builder.addMatcher(torqApi.endpoints.deleteTableView.matchFulfilled, (state, { payload }) => {
//       state.onChainViews = [
//         ...state.onChainViews.slice(0, payload.index),
//         ...state.onChainViews.slice(payload.index + 1, state.onChainViews.length),
//       ];
//       state.selectedViewIndex = 0;
//     });
//
//     builder.addMatcher(torqApi.endpoints.getTableViews.matchFulfilled, (state, { payload }) => {
//       if (payload !== null) {
//         state.onChainViews = payload.map((view: { id: number; view: ViewInterface<OnChainTx> }) => {
//           return { ...view.view, id: view.id };
//         });
//       }
//     });
//
//     builder.addMatcher(torqApi.endpoints.updateTableView.matchFulfilled, (state, { payload }) => {
//       const view = state.onChainViews.find((v) => v.id === payload.id);
//       if (view) {
//         view.saved = true;
//       }
//     });
//   },
// });
//
// export const { updateOnChainFilters, updateColumns, updateViews, updateViewsOrder, deleteView, updateSelectedView } =
//   onChainSlice.actions;
//
// export const selectOnChainFilters = (state: { onChain: TableOnChainState }) => {
//   return state.onChain.onChainViews[state.onChain.selectedViewIndex].filters;
// };
//
// export const selectActiveColumns = (state: { onChain: TableOnChainState }) => {
//   return state.onChain.onChainViews[state.onChain.selectedViewIndex].columns || [];
// };
//
// export const selectAllColumns = (_: { onChain: TableOnChainState }) => AllOnChainColumns;
// export const selectViews = (state: { onChain: TableOnChainState }) => state.onChain.onChainViews;
// export const selectCurrentView = (state: { onChain: TableOnChainState }) =>
//   state.onChain.onChainViews[state.onChain.selectedViewIndex];
// export const selectedViewIndex = (state: { onChain: TableOnChainState }) => state.onChain.selectedViewIndex;
//
// export default onChainSlice.reducer;
