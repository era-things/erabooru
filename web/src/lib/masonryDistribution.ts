interface Item {
    height: number; // Assuming each item has a height property
}

export function distributeRoundRobin<T>(items: T[], columnCount: number): T[][] {
    const columns: T[][] = Array.from({ length: columnCount }, () => []);
    items.forEach((item, i) => {
        columns[i % columnCount].push(item);
    });
    return columns;
}

export function distributeByHeight<T extends Item>(items: T[], columnCount: number): T[][] {
    const columns: T[][] = Array.from({ length: columnCount }, () => []);
    const columnHeights: number[] = Array(columnCount).fill(0);
    const columnsCount = Array(columnCount).fill(0);

    items.forEach((item) => {
        const {index: minCountIndex, value: minCount} = getMinIndexValue(columnsCount);
        const { index: minHeightIndex, value: minHeight } = getMinIndexValue(columnHeights);
        const { index: maxHeightIndex, value: maxHeight } = getMaxIndexValue(columnHeights);

        //distribute like round robin, but if one column is too short, put it there instead
        if( minHeight + item.height < maxHeight){
            columns[minHeightIndex].push(item);
            columnHeights[minHeightIndex] += item.height + 10; // Adding a gap of 10 pixels
            columnsCount[minHeightIndex] += 1;
        }
        else{
            columns[minCountIndex].push(item);
            columnHeights[minCountIndex] += item.height + 10; // Adding a gap of 10 pixels
            columnsCount[minCountIndex] += 1;
        }
    });

    return columns;
}

function getMinIndexValue(arr: number[]): {index: number, value: number} {
    let minIndex = 0;
    let minValue = arr[0];
    for (let i = 1; i < arr.length; i++) {
        if (arr[i] < minValue) {
            minValue = arr[i];
            minIndex = i;
        }
    }
    return { index: minIndex, value: minValue };
}

function getMaxIndexValue(arr: number[]): {index: number, value: number} {
    let maxIndex = 0;
    let maxValue = arr[0];
    for (let i = 1; i < arr.length; i++) {
        if (arr[i] > maxValue) {
            maxValue = arr[i];
            maxIndex = i;
        }
    }
    return { index: maxIndex, value: maxValue };
}