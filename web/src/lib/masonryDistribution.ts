interface Item {
    height: number; // Assuming each item has a height property
    width: number; // Assuming each item has a width property
}

export function distributeRoundRobin<T>(items: T[], columnCount: number): T[][] {
    const columns: T[][] = Array.from({ length: columnCount }, () => []);
    items.forEach((item, i) => {
        columns[i % columnCount].push(item);
    });
    return columns;
}

export function distributeHybrid<T extends Item>(items: T[], columnCount: number): T[][] {
    items = normalize(items, 90); // Normalize items to a fixed width
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

export function distributeByHeight<T extends Item>(items: T[], columnCount: number): T[][] {
    items = normalize(items, 90); // Normalize items to a fixed width
    const columns: T[][] = Array.from({ length: columnCount }, () => []);
    const columnHeights: number[] = Array(columnCount).fill(0);

    items.forEach((item) => {
        const { index: minIndex, value: minHeight } = getMinIndexValue(columnHeights);
        columns[minIndex].push(item);
        columnHeights[minIndex] += item.height + 10; // Adding a gap of 10 pixels
    });

    return columns;
}

export function distributeVertically<T extends Item>(
  items: T[],
  columnCount: number,
  gap = 90,
): T[][] {
    items = normalize(items, gap);
    const n = items.length;
    //items max elem height
    let start = items.reduce((max, item) => Math.max(max, item.height), 0);
    //items sum of heights 
    let end = items.reduce((a, b) => a + b.height, 0);

    // ans stores possible maximum subarray sum
    let ans = 0;
    while (start <= end) {
        const mid = Math.floor((start + end) / 2);

        // If mid is possible solution, set ans = mid
        if (check(mid, items, columnCount)) {
            ans = mid;
            end = mid - (10);
        }
        else {
            start = mid + (10);
        }
    }

    // knowing the answer, we can construct the columns
    const columns: T[][] = Array.from({ length: columnCount }, () => []);
    let count = 0;
    let accum = 0;
    let currentColumn = 0;
    while(currentColumn < columnCount && count < n) {
        const itemHeight = items[count].height;
        if (accum + itemHeight  > ans) {
            currentColumn++;
            accum = 0;
        }
        accum += itemHeight; 
        columns[currentColumn].push(items[count]);
        count++;
    }

    return columns;
}

function check<T extends Item>(mid: number, arr: T[], k: number): boolean {
    const n = arr.length;
    let count = 0;
    let sum = 0;

    for (let i = 0; i < n; i++) {
    
        // If individual element is greater
        // maximum possible sum
        if (arr[i].height > mid) {
            return false;
        }

        // Increase sum of current sub-array
        sum += arr[i].height;

        // If the sum is greater than mid, increase count
        if (sum > mid) {
            count++;
            sum = arr[i].height;
        }
    }
    count++;

    return count <= k;
}

function normalize<T extends Item>(items: T[], gap: number): T[] {
    return items.map((item, index) => { 
        //scale needed to make width = 1000
        const scaleNeeded = 1000 / item.width;
        return {
            ...item,
            height: Math.floor(item.height * scaleNeeded) + gap,
            width: Math.floor(item.width * scaleNeeded)
        };
    });
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