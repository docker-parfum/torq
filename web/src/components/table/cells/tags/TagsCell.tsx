import classNames from "classnames";
import cellStyles from "components/table/cells/cell.module.scss";
import styles from "./tags_cell.module.scss";
import { Link, useLocation } from "react-router-dom";
import { Tag as TagType } from "pages/tags/tagsTypes";
import Tag, { TagSize } from "components/tags/Tag";
import { ColorVariant, LinkButton, SizeVariant } from "components/buttons/Button";
import { MoleculeRegular as NodeIcon, ArrowRoutingRegular as ChannelsIcon } from "@fluentui/react-icons";

export type TagsCellProps = {
  channelId: number;
  nodeId: number;
  tags: TagType[];
  totalCell?: boolean;
};

function EditLinkWrapper(props: { tag: TagType }) {
  const location = useLocation();

  return (
    <Link
      to={`/update-tag/${props.tag.tagId}`}
      state={{ background: location }}
      className={classNames(cellStyles.action, styles.updateLink)}
    >
      <Tag label={props.tag.name} colorVariant={props.tag.style} sizeVariant={TagSize.tiny} />
    </Link>
  );
}

const TagsCell = (props: TagsCellProps) => {
  const location = useLocation();
  return (
    <div className={classNames(cellStyles.cell, styles.tagCell, { [cellStyles.totalCell]: props.totalCell })}>
      {!props.totalCell && (props.tags || []).map((tag) => <EditLinkWrapper tag={tag} key={"tag-" + tag.tagId} />)}

      {!props.totalCell && (
        <>
          <LinkButton
            to={`/tag-channel/${props.channelId}`}
            state={{ background: location }}
            icon={<ChannelsIcon />}
            buttonSize={SizeVariant.tiny}
            buttonColor={ColorVariant.disabled}
          />
          <LinkButton
            to={`/tag-node/${props.nodeId}`}
            state={{ background: location }}
            icon={<NodeIcon />}
            buttonSize={SizeVariant.tiny}
            buttonColor={ColorVariant.disabled}
          />
        </>
      )}
    </div>
  );
};

export default TagsCell;
