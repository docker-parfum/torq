import styles from "components/table/cells/cell.module.scss";
import { ColumnMetaData } from "features/table/types";
import DefaultCellRenderer from "features/table/DefaultCellRenderer";
import AliasCell from "components/table/cells/alias/AliasCell";
import { Forward } from "./forwardsTypes";
import { SelectChannelTags } from "apiSlice";
import { useAppSelector } from "store/hooks";
import { selectActiveNetwork } from "../network/networkSlice";
import { useSelector } from "react-redux";
import TagsCell from "components/table/cells/tags/TagsCell";

export default function channelsCellRenderer(
  row: Forward,
  rowIndex: number,
  column: ColumnMetaData<Forward>,
  columnIndex: number,
  isTotalsRow?: boolean,
  maxRow?: Forward
): JSX.Element {
  if (column.key === "alias") {
    return (
      <AliasCell
        current={row["alias"] as string}
        channelId={row.channelId}
        nodeIds={row.localNodeIds}
        open={row["open"]}
        key={"alias" + rowIndex + columnIndex}
        className={column.locked ? styles.locked : ""}
        isTotalsRow={isTotalsRow}
      />
    );
  }

  if (column.key === "tags") {
    const activeNetwork = useAppSelector(selectActiveNetwork);
    const tags = useSelector(SelectChannelTags({ channelId: row.channelId, network: activeNetwork }));
    return (
      <TagsCell
        tags={tags.tags || []}
        key={"tags" + rowIndex + columnIndex}
        channelId={row.channelId}
        nodeId={row.secondNodeId}
        totalCell={isTotalsRow}
      />
    );
  }

  // Use the defualt
  return DefaultCellRenderer(row, rowIndex, column, columnIndex, isTotalsRow, maxRow);
}
