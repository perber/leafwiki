import { useMeasure } from '@/lib/useMeasure';
import { useTreeStore } from '@/stores/tree';
import { ChevronUp, File } from 'lucide-react';
import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { PageNode } from '../../lib/api';
import { AddPageDialog } from '../page/AddPageDialog';
import { MovePageButton } from '../page/MovePageButton';
import { SortPagesDialog } from '../page/SortPagesDialog';

type Props = {
  node: PageNode;
  level?: number;
};

export function TreeNode({ node, level = 0 }: Props) {
  const { isNodeOpen, toggleNode, searchQuery } = useTreeStore();
  const hasChildren = node.children && node.children.length > 0;
  const [hovered, setHovered] = useState(false);
  const { pathname } = useLocation();
  const isActive = `/${node.path}` === pathname;
  const open = isNodeOpen(node.id);
  
  const [ref] = useMeasure<HTMLDivElement>();

  const highlightTitle = () => {
    if (!searchQuery) return node.title;

    const index = node.title.toLowerCase().indexOf(searchQuery.toLowerCase());
    if (index === -1) return node.title;

    const before = node.title.slice(0, index);
    const match = node.title.slice(index, index + searchQuery.length);
    const after = node.title.slice(index + searchQuery.length);

    return (
      <>
        {before}
        <mark className="bg-yellow-200 text-black">{match}</mark>
        {after}
      </>
    );
  };

  const linkText = (
    <Link to={`/${node.path}`}>
      <span className="block w-[150px] overflow-hidden truncate text-ellipsis">
        {highlightTitle()}
      </span>
    </Link>
  );

  return (
    <div>
      <div
        className={`flex cursor-pointer items-center text-base transition-all ease-in-out duration-200 rounded-lg pt-1 pb-1 ${
          isActive ? 'bg-gray-200 font-semibold' : 'hover:bg-gray-100 text-gray-800'
        }`}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <div className="flex items-center flex-1 gap-2">
          {hasChildren && (
            <ChevronUp
              size={16}
              className={`transition-transform ${open ? 'rotate-180' : 'rotate-90'}`}
              onClick={() => hasChildren && toggleNode(node.id)}
            />
          )}

          {/* Zeigt das File-Icon für Knoten ohne Kinder */}
          {!hasChildren && <File size={18} className="text-gray-400" />}
          
          {linkText}
        </div>

        {hovered && (
          <div className="flex gap-0">
            <AddPageDialog parentId={node.id} minimal />
            <MovePageButton pageId={node.id} />
            {hasChildren && <SortPagesDialog parent={node} />}
          </div>
        )}
      </div>

      {/* Animierter Bereich für Kinderknoten */}
      <div
        ref={ref}
        className="ml-4 transition-[max-height, opacity] duration-500 ease-in-out overflow-hidden"
        style={{
          maxHeight: open ? `1000px` : '0px',
          opacity: open ? 1 : 0,
        }}
      >
        {hasChildren &&
          node.children.map((child) => (
            <TreeNode key={child.id} node={child} level={level + 1} />
          ))}
      </div>
    </div>
  );
}
