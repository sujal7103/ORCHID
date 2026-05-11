import type { Node, Edge } from '@xyflow/react';

interface LayoutOptions {
  rankSep?: number; // Horizontal spacing between columns
  nodeSep?: number; // Vertical spacing between nodes in same column
  marginX?: number;
  marginY?: number;
}

// Check if a node is the Start block
function isStartBlock(node: Node): boolean {
  const block = node.data?.block;
  if (!block) return false;
  return (
    block.type === 'variable' &&
    block.config?.operation === 'read' &&
    block.config?.variableName === 'input'
  );
}

// Get node dimensions
function getNodeDimensions(node: Node): { width: number; height: number } {
  if (isStartBlock(node)) {
    return { width: 280, height: 320 };
  }
  return { width: 280, height: 130 };
}

// Build parent-child relationships from edges
function buildGraph(
  nodes: Node[],
  edges: Edge[]
): {
  children: Map<string, string[]>;
  parents: Map<string, string[]>;
  roots: string[];
  nodeMap: Map<string, Node>;
} {
  const children = new Map<string, string[]>();
  const parents = new Map<string, string[]>();
  const nodeMap = new Map<string, Node>();

  nodes.forEach(n => {
    children.set(n.id, []);
    parents.set(n.id, []);
    nodeMap.set(n.id, n);
  });

  edges.forEach(edge => {
    const childList = children.get(edge.source) || [];
    if (!childList.includes(edge.target)) {
      childList.push(edge.target);
      children.set(edge.source, childList);
    }

    const parentList = parents.get(edge.target) || [];
    if (!parentList.includes(edge.source)) {
      parentList.push(edge.source);
      parents.set(edge.target, parentList);
    }
  });

  // Find roots - prioritize Start blocks
  const allRoots = nodes.filter(n => (parents.get(n.id) || []).length === 0);
  const startBlocks = allRoots.filter(isStartBlock);
  const roots = startBlocks.length > 0 ? startBlocks : allRoots;

  return { children, parents, roots: roots.map(n => n.id), nodeMap };
}

// Calculate depths - node depth = max depth of all parents + 1
// This ensures nodes are placed in the column after ALL their dependencies
function calculateDepths(
  nodes: Node[],
  graph: { children: Map<string, string[]>; parents: Map<string, string[]>; roots: string[] }
): Map<string, number> {
  const depths = new Map<string, number>();
  const visited = new Set<string>();

  // Recursive function to calculate depth
  function getDepth(nodeId: string): number {
    if (depths.has(nodeId)) return depths.get(nodeId)!;

    // Prevent infinite loops in case of cycles
    if (visited.has(nodeId)) return 0;
    visited.add(nodeId);

    const parentIds = graph.parents.get(nodeId) || [];

    if (parentIds.length === 0) {
      // Root node
      depths.set(nodeId, 0);
      return 0;
    }

    // Depth is 1 + max depth of all parents
    const maxParentDepth = Math.max(...parentIds.map(pid => getDepth(pid)));
    const depth = maxParentDepth + 1;
    depths.set(nodeId, depth);
    return depth;
  }

  // Calculate depth for all nodes
  nodes.forEach(n => getDepth(n.id));

  return depths;
}

// Group nodes by depth level
function groupByDepth(nodes: Node[], depths: Map<string, number>): Map<number, Node[]> {
  const groups = new Map<number, Node[]>();

  nodes.forEach(node => {
    const depth = depths.get(node.id) || 0;
    if (!groups.has(depth)) groups.set(depth, []);
    groups.get(depth)!.push(node);
  });

  return groups;
}

// Sort nodes within columns to minimize edge crossings
function optimizeNodeOrder(
  nodesByDepth: Map<number, Node[]>,
  graph: { children: Map<string, string[]>; parents: Map<string, string[]> },
  maxDepth: number
): Map<number, Node[]> {
  const positions = new Map<string, number>();

  // Initialize positions for first column
  const firstColumn = nodesByDepth.get(0) || [];
  firstColumn.forEach((node, idx) => positions.set(node.id, idx));

  // Forward pass: order by parent positions
  for (let d = 1; d <= maxDepth; d++) {
    const column = nodesByDepth.get(d) || [];

    const scored = column.map(node => {
      const parentIds = graph.parents.get(node.id) || [];
      if (parentIds.length === 0) return { node, score: 0 };

      // Average position of parents
      const sum = parentIds.reduce((acc, pid) => acc + (positions.get(pid) ?? 0), 0);
      return { node, score: sum / parentIds.length };
    });

    scored.sort((a, b) => a.score - b.score);
    scored.forEach(({ node }, idx) => positions.set(node.id, idx));
    nodesByDepth.set(
      d,
      scored.map(s => s.node)
    );
  }

  // Backward pass: refine based on children
  for (let d = maxDepth - 1; d >= 0; d--) {
    const column = nodesByDepth.get(d) || [];

    const scored = column.map(node => {
      const childIds = graph.children.get(node.id) || [];
      const parentScore = positions.get(node.id) ?? 0;

      if (childIds.length === 0) return { node, score: parentScore };

      // Average position of children
      const sum = childIds.reduce((acc, cid) => acc + (positions.get(cid) ?? 0), 0);
      const childScore = sum / childIds.length;

      // Blend parent and child scores
      return { node, score: (parentScore + childScore) / 2 };
    });

    scored.sort((a, b) => a.score - b.score);
    scored.forEach(({ node }, idx) => positions.set(node.id, idx));
    nodesByDepth.set(
      d,
      scored.map(s => s.node)
    );
  }

  return nodesByDepth;
}

/**
 * n8n-style Left-to-Right layout
 * - Clean horizontal flow from left to right
 * - Nodes aligned in vertical columns by depth
 * - Each column centered vertically
 * - Nodes ordered to minimize edge crossings
 */
export function getLayoutedElements(
  nodes: Node[],
  edges: Edge[],
  options: LayoutOptions = {}
): { nodes: Node[]; edges: Edge[] } {
  if (nodes.length === 0) return { nodes: [], edges };

  const {
    marginX = 50,
    marginY = 50,
    rankSep = 100, // Horizontal gap between columns
    nodeSep = 60, // Vertical gap between nodes in same column
  } = options;

  // Build graph and calculate depths
  const graph = buildGraph(nodes, edges);
  const depths = calculateDepths(nodes, graph);
  const maxDepth = Math.max(...Array.from(depths.values()), 0);

  // Group nodes by depth (column)
  let nodesByDepth = groupByDepth(nodes, depths);

  // Optimize node order within columns
  nodesByDepth = optimizeNodeOrder(nodesByDepth, graph, maxDepth);

  // Get dimensions for all nodes
  const dimensions = new Map<string, { width: number; height: number }>();
  nodes.forEach(n => dimensions.set(n.id, getNodeDimensions(n)));

  // Calculate column widths and heights
  const columnWidths = new Map<number, number>();
  const columnHeights = new Map<number, number>();

  for (let d = 0; d <= maxDepth; d++) {
    const column = nodesByDepth.get(d) || [];
    let maxWidth = 0;
    let totalHeight = 0;

    column.forEach((node, i) => {
      const dims = dimensions.get(node.id) || { width: 280, height: 130 };
      maxWidth = Math.max(maxWidth, dims.width);
      totalHeight += dims.height;
      if (i < column.length - 1) totalHeight += nodeSep;
    });

    columnWidths.set(d, maxWidth);
    columnHeights.set(d, totalHeight);
  }

  // Find max column height for centering
  let maxColumnHeight = 0;
  columnHeights.forEach(h => {
    maxColumnHeight = Math.max(maxColumnHeight, h);
  });

  // Calculate X position for each column (left edge of column)
  const columnX = new Map<number, number>();
  let currentX = marginX;

  for (let d = 0; d <= maxDepth; d++) {
    columnX.set(d, currentX);
    const width = columnWidths.get(d) || 280;
    currentX += width + rankSep;
  }

  // Position nodes - each column centered vertically
  const positionedNodes: Node[] = [];

  for (let d = 0; d <= maxDepth; d++) {
    const column = nodesByDepth.get(d) || [];
    const columnHeight = columnHeights.get(d) || 0;
    const x = columnX.get(d) || 0;

    // Center this column vertically
    const startY = marginY + (maxColumnHeight - columnHeight) / 2;

    let currentY = startY;
    column.forEach(node => {
      const dims = dimensions.get(node.id) || { width: 280, height: 130 };

      positionedNodes.push({
        ...node,
        position: { x, y: currentY },
      });

      currentY += dims.height + nodeSep;
    });
  }

  return { nodes: positionedNodes, edges };
}

/**
 * Compact grid layout for very large workflows
 */
export function getCompactGridLayout(
  nodes: Node[],
  edges: Edge[],
  options: { columns?: number; cellWidth?: number; cellHeight?: number } = {}
): { nodes: Node[]; edges: Edge[] } {
  if (nodes.length === 0) return { nodes: [], edges };

  const cols = options.columns ?? Math.min(Math.ceil(Math.sqrt(nodes.length * 1.5)), 8);
  const cellWidth = options.cellWidth ?? 320;
  const cellHeight = options.cellHeight ?? 160;
  const margin = 50;

  const graph = buildGraph(nodes, edges);
  const depths = calculateDepths(nodes, graph);

  const sortedNodes = [...nodes].sort((a, b) => {
    const depthA = depths.get(a.id) || 0;
    const depthB = depths.get(b.id) || 0;
    return depthA - depthB;
  });

  const positionedNodes = sortedNodes.map((node, i) => {
    const row = Math.floor(i / cols);
    const col = i % cols;

    return {
      ...node,
      position: {
        x: margin + col * cellWidth,
        y: margin + row * cellHeight,
      },
    };
  });

  return { nodes: positionedNodes, edges };
}

/**
 * Smart layout selector
 */
export function getSmartLayout(
  nodes: Node[],
  edges: Edge[],
  options: LayoutOptions = {}
): { nodes: Node[]; edges: Edge[] } {
  if (nodes.length === 0) return { nodes: [], edges };

  if (nodes.length > 50) {
    return getCompactGridLayout(nodes, edges);
  }

  return getLayoutedElements(nodes, edges, options);
}
